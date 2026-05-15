package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const piSessionRootDir = "pi-sessions"

// piExecutionMode is the interaction pattern sent to Pi for a given invocation.
// Doug selects the mode through piExecutionModeFor so that new modes can be
// introduced without changing any Doug call site.
type piExecutionMode string

const (
	// piExecutionModeOneShot is the one-prompt/one-agent_end interaction pattern.
	// All current Doug workflow phases use this mode.
	piExecutionModeOneShot piExecutionMode = "one_shot"
)

// PiAdapter is the Doug-owned Backend for Pi RPC runs. When execution_mode is
// "rpc", PiAdapter is the required execution boundary — Doug routes all agent
// invocations through Pi, which owns model selection, tool enforcement, and
// agent process lifecycle. Command handlers continue to speak only in terms of
// RunRequest/RunResponse; Pi-specific request preparation remains private to
// internal/agent.
type PiAdapter struct {
	launcher piLauncher
}

type piLauncher interface {
	Run(ctx context.Context, spec piLaunchSpec) (RunResponse, error)
}

type piLaunchSpec struct {
	WorkingDir        string
	Request           piRPCRequest
	Lifecycle         LifecycleHooks
	HeartbeatInterval time.Duration
	HeartbeatFn       func(elapsed time.Duration)
	Output            io.Writer
}

type piRPCRequest struct {
	Phase        string
	Execution    piRPCExecution
	Session      piRPCSession
	Task         piRPCTask
	Brief        piRPCBrief
	Context      []piRPCContextInput
	Artifacts    piRPCArtifacts
	Routing      piRPCRouting
	Policy       piRPCPolicy
	Restrictions piRPCRestrictions
}

type piRPCExecution struct {
	Mode    string
	Command string
}

type piRPCSession struct {
	Mode      string
	Directory string
}

type piRPCTask struct {
	ID         string
	Type       string
	Attempt    int
	MaxRetries int
	EpicID     string
	EpicName   string
}

type piRPCBrief struct {
	Path      string
	Format    string
	Authority string
}

type piRPCContextInput struct {
	Kind      string
	Path      string
	Required  bool
	Authority string
}

type piRPCArtifacts struct {
	Read  []piRPCArtifactSurface
	Write []piRPCArtifactSurface
}

type piRPCArtifactSurface struct {
	Path        string
	Purpose     string
	Authority   string
	AgentFacing bool
}

type piRPCRouting struct {
	Workflow  string
	SkillName string
}

type piRPCPolicy struct {
	SessionPolicy string
}

type piRPCRestrictions struct {
	Read  piRPCRestrictionHook `json:"read"`
	Write piRPCRestrictionHook `json:"write"`
}

type piRPCRestrictionHook struct {
	Mode  string   `json:"mode"`
	Paths []string `json:"paths,omitempty"`
}

// piExecutionModeFor returns the Pi interaction mode for a given RunRequest.
// All current Doug workflow phases use piExecutionModeOneShot. Future phases
// that require a different interaction pattern add a case here; no call site
// outside internal/agent changes.
func piExecutionModeFor(_ RunRequest) piExecutionMode {
	return piExecutionModeOneShot
}

// NewPiAdapter constructs a Pi-backed backend boundary. The launcher is kept
// private so future Pi protocol work can evolve without changing call sites.
func NewPiAdapter() PiAdapter {
	return PiAdapter{launcher: piCLILauncher{command: "pi"}}
}

// Run implements Backend by translating the Doug-native request into a
// Pi-owned launch spec, then delegating the Pi-specific work to a private
// launcher.
func (a PiAdapter) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	launcher := a.launcher
	if launcher == nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("pi adapter launcher is not configured")
	}

	return launcher.Run(ctx, piLaunchSpec{
		WorkingDir:        req.ProjectRoot,
		Request:           buildPiRPCRequest(req),
		Lifecycle:         req.Lifecycle,
		HeartbeatInterval: req.HeartbeatInterval,
		HeartbeatFn:       req.HeartbeatFn,
		Output:            req.Output,
	})
}

func buildPiRPCRequest(req RunRequest) piRPCRequest {
	return piRPCRequest{
		Phase: phaseSessionComponent(req.Phase),
		Execution: piRPCExecution{
			Mode:    string(piExecutionModeFor(req)),
			Command: req.Command,
		},
		Session: piRPCSession{
			Mode:      "retain",
			Directory: piSessionDir(req),
		},
		Task: piRPCTask{
			ID:         req.Task.ID,
			Type:       req.Task.Type,
			Attempt:    req.Task.Attempt,
			MaxRetries: req.Task.MaxRetries,
			EpicID:     req.Task.EpicID,
			EpicName:   req.Task.EpicName,
		},
		Brief: piRPCBrief{
			Path:      req.Brief.Path,
			Format:    string(req.Brief.Format),
			Authority: string(req.Brief.Authority),
		},
		Context:      mapPiContextInputs(req.ContextLoadOrder),
		Artifacts:    mapPiArtifacts(req.Artifacts),
		Routing:      piRPCRouting{Workflow: req.Routing.Workflow, SkillName: req.Routing.SkillName},
		Policy:       piRPCPolicy{SessionPolicy: req.Policy.SessionPolicy},
		Restrictions: mapPiRestrictions(req.Restrictions),
	}
}

func mapPiContextInputs(inputs []ContextInput) []piRPCContextInput {
	if len(inputs) == 0 {
		return nil
	}

	mapped := make([]piRPCContextInput, 0, len(inputs))
	for _, input := range inputs {
		mapped = append(mapped, piRPCContextInput{
			Kind:      string(input.Kind),
			Path:      input.Path,
			Required:  input.Required,
			Authority: string(input.Authority),
		})
	}
	return mapped
}

func mapPiArtifacts(artifacts ArtifactSurfaces) piRPCArtifacts {
	return piRPCArtifacts{
		Read:  mapPiArtifactSurfaces(artifacts.Read),
		Write: mapPiArtifactSurfaces(artifacts.Write),
	}
}

func mapPiArtifactSurfaces(surfaces []ArtifactSurface) []piRPCArtifactSurface {
	if len(surfaces) == 0 {
		return nil
	}

	mapped := make([]piRPCArtifactSurface, 0, len(surfaces))
	for _, surface := range surfaces {
		mapped = append(mapped, piRPCArtifactSurface{
			Path:        surface.Path,
			Purpose:     string(surface.Purpose),
			Authority:   string(surface.Authority),
			AgentFacing: surface.AgentFacing,
		})
	}
	return mapped
}

func mapPiRestrictions(restrictions RestrictionHooks) piRPCRestrictions {
	return piRPCRestrictions{
		Read: piRPCRestrictionHook{
			Mode:  string(restrictions.Read.Mode),
			Paths: cloneStrings(restrictions.Read.Paths),
		},
		Write: piRPCRestrictionHook{
			Mode:  string(restrictions.Write.Mode),
			Paths: cloneStrings(restrictions.Write.Paths),
		},
	}
}

// buildPiPromptPayload assembles the prompt RPC message sent to Pi. When
// restrictions are configured, they are included so Pi can apply them directly;
// this is the Phase 1 enforcement layer for Doug-selected runtime restrictions.
func buildPiPromptPayload(id, message string, restrictions piRPCRestrictions) map[string]any {
	payload := map[string]any{
		"id":      id,
		"type":    "prompt",
		"message": message,
	}
	if restrictions.Read.Mode != "" || len(restrictions.Read.Paths) > 0 ||
		restrictions.Write.Mode != "" || len(restrictions.Write.Paths) > 0 {
		payload["restrictions"] = restrictions
	}
	return payload
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func piSessionDir(req RunRequest) string {
	epicID := req.Task.EpicID
	if epicID == "" {
		epicID = phaseSessionComponent(req.Phase)
	}

	taskID := req.Task.ID
	if taskID == "" {
		taskID = "task"
	}

	attemptDir := "attempt-0"
	if req.Task.Attempt > 0 {
		attemptDir = fmt.Sprintf("attempt-%d", req.Task.Attempt)
	}

	return filepath.Join(req.ProjectRoot, ".doug", "logs", piSessionRootDir, epicID, taskID, attemptDir)
}

func phaseSessionComponent(p RunPhase) string {
	if p == "" {
		return "runtime"
	}
	return string(p)
}

type piCLILauncher struct {
	command    string
	baseArgs   []string
	newCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func (l piCLILauncher) Run(ctx context.Context, spec piLaunchSpec) (RunResponse, error) {
	if spec.WorkingDir == "" {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("pi working directory is required")
	}
	if spec.Request.Session.Directory == "" {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("pi session directory is required")
	}

	if err := os.MkdirAll(spec.Request.Session.Directory, 0o755); err != nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("create pi session directory: %w", err)
	}

	command := l.command
	if command == "" {
		command = "pi"
	}
	newCommand := l.newCommand
	if newCommand == nil {
		newCommand = exec.CommandContext
	}

	args := append([]string{}, l.baseArgs...)
	args = append(args, "--mode", "rpc", "--session-dir", spec.Request.Session.Directory)
	cmd := newCommand(ctx, command, args...)
	cmd.Dir = spec.WorkingDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("open pi stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("open pi stdout: %w", err)
	}

	stderr := &piStderrWriter{forward: spec.Output}
	cmd.Stderr = stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("start pi rpc process: %w", err)
	}

	done := make(chan struct{})
	defer close(done)

	if spec.HeartbeatInterval > 0 && spec.HeartbeatFn != nil {
		ticker := time.NewTicker(spec.HeartbeatInterval)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-ticker.C:
					spec.HeartbeatFn(time.Since(start))
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			}
		}()
	}

	lines := make(chan piRPCEnvelope)
	readErrs := make(chan error, 1)
	go readPiJSONL(stdout, lines, readErrs, spec.Output)

	obs := newPiRunObservability()
	sessionID, err := l.runInteraction(ctx, stdin, lines, readErrs, spec.Request, obs)
	closeErr := stdin.Close()

	waitErr := cmd.Wait()
	duration := time.Since(start)

	resp := RunResponse{
		Status:              RunStatusCompleted,
		Duration:            duration,
		SessionID:           sessionID,
		AvailableSessionIDs: obs.sessionIDs(),
	}

	if ctx.Err() != nil {
		resp.Status = RunStatusCancelled
		fireLifecycleHooks(spec.Lifecycle, duration, ctx.Err())
		return resp, ctx.Err()
	}
	if err != nil {
		resp.Status = RunStatusRejected
		if waitErr == nil && closeErr != nil {
			err = fmt.Errorf("%w (close stdin: %v)", err, closeErr)
		}
		return resp, err
	}
	if closeErr != nil {
		resp.Status = RunStatusRejected
		return resp, fmt.Errorf("close pi stdin: %w", closeErr)
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			resp.ExitCode = &code
			return resp, fmt.Errorf("pi exited with code %d", code)
		}
		resp.Status = RunStatusRejected
		return resp, fmt.Errorf("wait for pi rpc process: %w", waitErr)
	}

	code := 0
	resp.ExitCode = &code
	return resp, nil
}

// runInteraction dispatches to the correct Pi interaction implementation based
// on the mode carried in req.Execution.Mode. Adding a new interaction pattern
// requires a new piExecutionMode constant, a new runXxxInteraction method, and
// a new case here — no Doug call sites outside internal/agent change.
func (l piCLILauncher) runInteraction(
	ctx context.Context,
	stdin io.Writer,
	lines <-chan piRPCEnvelope,
	readErrs <-chan error,
	req piRPCRequest,
	obs *piRunObservability,
) (string, error) {
	return l.runOneShotInteraction(ctx, stdin, lines, readErrs, req, obs)
}

func (l piCLILauncher) runOneShotInteraction(
	ctx context.Context,
	stdin io.Writer,
	lines <-chan piRPCEnvelope,
	readErrs <-chan error,
	req piRPCRequest,
	obs *piRunObservability,
) (string, error) {
	const stateRequestID = "doug-startup"
	if err := writePiJSONL(stdin, map[string]any{
		"id":   stateRequestID,
		"type": "get_state",
	}); err != nil {
		return "", fmt.Errorf("request pi startup state: %w", err)
	}

	sessionID, err := awaitPiState(ctx, lines, readErrs, stateRequestID, obs)
	if err != nil {
		return "", err
	}

	message := req.Execution.Command
	if message == "" {
		return sessionID, nil
	}

	const promptRequestID = "doug-prompt"
	if err := writePiJSONL(stdin, buildPiPromptPayload(promptRequestID, message, req.Restrictions)); err != nil {
		return sessionID, fmt.Errorf("send pi prompt: %w", err)
	}

	if err := awaitPiPromptCompletion(ctx, lines, readErrs, promptRequestID, obs); err != nil {
		return sessionID, err
	}

	return sessionID, nil
}

type piRPCEnvelope struct {
	Type    string         `json:"type"`
	ID      string         `json:"id"`
	Command string         `json:"command"`
	Success bool           `json:"success"`
	Error   string         `json:"error"`
	Data    map[string]any `json:"data"`
	Raw     map[string]any `json:"-"`
	RawLine string         `json:"-"`
}

func readPiJSONL(r io.Reader, out chan<- piRPCEnvelope, errs chan<- error, mirror io.Writer) {
	defer close(out)

	scanner := bufio.NewScanner(r)
	const maxJSONLLine = 8 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxJSONLLine)

	for scanner.Scan() {
		rawLine := scanner.Text()
		if mirror != nil {
			if _, err := fmt.Fprintf(mirror, "pi rpc stdout: %s\n", rawLine); err != nil {
				errs <- fmt.Errorf("mirror pi rpc stdout: %w", err)
				return
			}
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(rawLine), &raw); err != nil {
			errs <- fmt.Errorf("decode pi rpc line: %w", err)
			return
		}

		envelope := piRPCEnvelope{Raw: raw, RawLine: rawLine}
		if value, ok := raw["type"].(string); ok {
			envelope.Type = value
		}
		if value, ok := raw["id"].(string); ok {
			envelope.ID = value
		}
		if value, ok := raw["command"].(string); ok {
			envelope.Command = value
		}
		if value, ok := raw["success"].(bool); ok {
			envelope.Success = value
		}
		if value, ok := raw["error"].(string); ok {
			envelope.Error = value
		}
		if value, ok := raw["data"].(map[string]any); ok {
			envelope.Data = value
		}
		out <- envelope
	}

	if err := scanner.Err(); err != nil {
		errs <- fmt.Errorf("read pi rpc stdout: %w", err)
	}
}

func writePiJSONL(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func awaitPiState(ctx context.Context, lines <-chan piRPCEnvelope, readErrs <-chan error, requestID string, obs *piRunObservability) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case err := <-readErrs:
			if err != nil {
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				return "", err
			}
		case line, ok := <-lines:
			if !ok {
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				return "", fmt.Errorf("pi rpc stdout closed before startup response")
			}
			obs.observe(line)
			if line.Type != "response" || line.ID != requestID {
				continue
			}
			if !line.Success {
				return "", fmt.Errorf("pi startup rejected get_state: %s", line.Error)
			}
			sessionID, _ := line.Data["sessionId"].(string)
			return sessionID, nil
		}
	}
}

func awaitPiPromptCompletion(ctx context.Context, lines <-chan piRPCEnvelope, readErrs <-chan error, requestID string, obs *piRunObservability) error {
	promptAccepted := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readErrs:
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return err
			}
		case line, ok := <-lines:
			if !ok {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if promptAccepted {
					return fmt.Errorf("pi rpc stdout closed before agent_end")
				}
				return fmt.Errorf("pi rpc stdout closed before prompt response")
			}
			obs.observe(line)
			if line.Type == "response" && line.ID == requestID {
				if !line.Success {
					return fmt.Errorf("pi prompt rejected: %s", line.Error)
				}
				promptAccepted = true
				continue
			}
			if promptAccepted && line.Type == "agent_end" {
				return nil
			}
		}
	}
}

type piRunObservability struct {
	seenSessionIDs map[string]struct{}
	orderedIDs     []string
}

func newPiRunObservability() *piRunObservability {
	return &piRunObservability{seenSessionIDs: make(map[string]struct{})}
}

func (o *piRunObservability) observe(line piRPCEnvelope) {
	if o == nil {
		return
	}
	o.collect(line.Raw)
}

func (o *piRunObservability) collect(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if normalizePiSessionKey(key) == "sessionid" {
				if id, ok := child.(string); ok && id != "" {
					o.addSessionID(id)
				}
			}
			o.collect(child)
		}
	case []any:
		for _, child := range v {
			o.collect(child)
		}
	}
}

func normalizePiSessionKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (o *piRunObservability) addSessionID(id string) {
	if _, ok := o.seenSessionIDs[id]; ok {
		return
	}
	o.seenSessionIDs[id] = struct{}{}
	o.orderedIDs = append(o.orderedIDs, id)
}

func (o *piRunObservability) sessionIDs() []string {
	if len(o.orderedIDs) == 0 {
		return nil
	}
	ids := make([]string, len(o.orderedIDs))
	copy(ids, o.orderedIDs)
	return ids
}

type piStderrWriter struct {
	forward io.Writer
	buffer  []byte
}

func (w *piStderrWriter) Write(p []byte) (int, error) {
	w.buffer = append(w.buffer, p...)
	if w.forward == nil {
		return len(p), nil
	}
	n, err := w.forward.Write(p)
	if n < len(p) && err == nil {
		err = io.ErrShortWrite
	}
	return len(p), err
}

func (w *piStderrWriter) String() string {
	return string(w.buffer)
}
