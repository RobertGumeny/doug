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
	"sync"
	"time"

	"github.com/robertgumeny/doug/internal/interactive"
	"github.com/robertgumeny/doug/internal/types"
)

const piSessionRootDir = "pi-sessions"

// piInteractionMode is the interaction pattern sent to Pi for a given invocation.
// Doug selects the mode through piInteractionModeFor so that new modes can be
// introduced without changing any Doug call site.
type piInteractionMode string

const (
	// piInteractionModeOneShot is the one-prompt/one-agent_end interaction pattern.
	piInteractionModeOneShot piInteractionMode = "one_shot"
	// piInteractionModeInteractive keeps the session interactive while a prompt is
	// running so Doug can answer Pi extension UI requests during planning.
	piInteractionModeInteractive piInteractionMode = "interactive"
)

// PiAdapter is the Doug-owned Backend for Pi RPC runs. PiAdapter is the required
// execution boundary: Doug routes agent invocations through Pi, which owns model
// selection, tool enforcement, and agent process lifecycle. CLI handlers continue
// to speak only in terms of RunRequest/RunResponse; Pi-specific request
// preparation remains private to internal/agent.
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
	HeartbeatFn       func(elapsed time.Duration, activity string)
	FirstResponseFn   func(elapsed time.Duration)
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
	Mode           string
	InitialMessage string
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

// piInteractionModeFor returns the Pi interaction mode for a Doug workflow
// phase. Unknown phases are rejected.
func piInteractionModeFor(req RunRequest) (piInteractionMode, error) {
	switch req.Phase {
	case RunPhasePlanning:
		return piInteractionModeInteractive, nil
	case RunPhaseRuntime, RunPhaseScaffold, RunPhaseResearch, RunPhasePostEpicKB:
		return piInteractionModeOneShot, nil
	default:
		return "", fmt.Errorf("unknown Doug workflow phase %q: no source-owned Pi routing is defined", req.Phase)
	}
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

	mode, err := piInteractionModeFor(req)
	if err != nil {
		return RunResponse{Status: RunStatusRejected}, err
	}

	return launcher.Run(ctx, piLaunchSpec{
		WorkingDir:        req.ProjectRoot,
		Request:           buildPiRPCRequest(req, mode),
		Lifecycle:         req.Lifecycle,
		HeartbeatInterval: req.HeartbeatInterval,
		HeartbeatFn:       req.HeartbeatFn,
		FirstResponseFn:   req.FirstResponseFn,
		Output:            req.Output,
	})
}

func buildPiRPCRequest(req RunRequest, mode piInteractionMode) piRPCRequest {
	return piRPCRequest{
		Phase: phaseSessionComponent(req.Phase),
		Execution: piRPCExecution{
			Mode:           string(mode),
			InitialMessage: req.InitialPrompt,
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

var (
	piIsInteractive = interactive.IsInteractive
	piNewPrompter   = func() interactive.Prompter { return interactive.New() }
)

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

	activity := newPiActivityTracker()
	if spec.HeartbeatInterval > 0 && spec.HeartbeatFn != nil {
		ticker := time.NewTicker(spec.HeartbeatInterval)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-ticker.C:
					spec.HeartbeatFn(time.Since(start), activity.String())
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
	go readPiJSONL(stdout, lines, readErrs, spec.Output, activity)

	obs := newPiRunObservability(start, spec.FirstResponseFn)
	sessionID, err := l.runInteraction(ctx, stdin, lines, readErrs, spec.Request, obs)
	closeErr := stdin.Close()

	waitErr := cmd.Wait()
	duration := time.Since(start)

	resp := RunResponse{
		Status:                 RunStatusCompleted,
		Duration:               duration,
		SessionID:              sessionID,
		AvailableSessionIDs:    obs.sessionIDs(),
		FirstResponseMs:        obs.firstResponseMs(),
		ToolCallCount:          obs.toolCallCount,
		ProviderFailures:       len(obs.providerFailures),
		ProviderFailureDetails: obs.providerFailureDetails(),
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		resp.ExitCode = &code
	}

	if ctx.Err() != nil {
		resp.Status = RunStatusCancelled
		fireLifecycleHooks(spec.Lifecycle, duration, ctx.Err())
		return resp, ctx.Err()
	}
	if err != nil {
		resp.Status = RunStatusRejected
		if isPiTransportFailureError(err) || (resp.ExitCode != nil && isKnownPiTransportFailure(stderr.String())) {
			resp.Status = RunStatusTransportFailure
		}
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
		if resp.ExitCode != nil {
			if isKnownPiTransportFailure(stderr.String()) {
				resp.Status = RunStatusTransportFailure
			}
			return resp, fmt.Errorf("pi exited with code %d", *resp.ExitCode)
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
// requires a new piInteractionMode constant, a new runXxxInteraction method, and
// a new case here — no Doug call sites outside internal/agent change.
func (l piCLILauncher) runInteraction(
	ctx context.Context,
	stdin io.Writer,
	lines <-chan piRPCEnvelope,
	readErrs <-chan error,
	req piRPCRequest,
	obs *piRunObservability,
) (string, error) {
	switch piInteractionMode(req.Execution.Mode) {
	case piInteractionModeInteractive:
		return l.runInteractiveInteraction(ctx, stdin, lines, readErrs, req, obs)
	default:
		return l.runOneShotInteraction(ctx, stdin, lines, readErrs, req, obs)
	}
}

func (l piCLILauncher) runOneShotInteraction(
	ctx context.Context,
	stdin io.Writer,
	lines <-chan piRPCEnvelope,
	readErrs <-chan error,
	req piRPCRequest,
	obs *piRunObservability,
) (string, error) {
	sessionID, err := startPiPrompt(ctx, stdin, lines, readErrs, req, obs)
	if err != nil {
		return "", err
	}
	if req.Execution.InitialMessage == "" {
		return sessionID, nil
	}
	if err := awaitPiPromptCompletion(ctx, lines, readErrs, "doug-prompt", obs); err != nil {
		return sessionID, err
	}
	return sessionID, nil
}

func (l piCLILauncher) runInteractiveInteraction(
	ctx context.Context,
	stdin io.Writer,
	lines <-chan piRPCEnvelope,
	readErrs <-chan error,
	req piRPCRequest,
	obs *piRunObservability,
) (string, error) {
	sessionID, err := startPiPrompt(ctx, stdin, lines, readErrs, req, obs)
	if err != nil {
		return "", err
	}
	if req.Execution.InitialMessage == "" {
		return sessionID, nil
	}
	if err := awaitPiInteractivePromptCompletion(ctx, stdin, lines, readErrs, "doug-prompt", obs); err != nil {
		return sessionID, err
	}
	return sessionID, nil
}

func startPiPrompt(
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

	message := req.Execution.InitialMessage
	if message == "" {
		return sessionID, nil
	}

	const promptRequestID = "doug-prompt"
	if err := writePiJSONL(stdin, buildPiPromptPayload(promptRequestID, message, req.Restrictions)); err != nil {
		return sessionID, fmt.Errorf("send pi prompt: %w", err)
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

func isPiTransportFailureError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "pi rpc stdout closed before agent_end") ||
		strings.Contains(text, "pi rpc stdout closed before prompt response") ||
		strings.Contains(text, "pi rpc stdout closed before startup response") ||
		strings.Contains(text, "read pi rpc stdout:") ||
		isKnownPiTransportFailure(text)
}

func isKnownPiTransportFailure(text string) bool {
	text = strings.ToLower(text)
	patterns := []string{
		"provider_transport_failure",
		"transport_failure",
		"transport failure",
		"transport error",
		"websocket",
		"web socket",
		"connection reset",
		"connection refused",
		"connection closed",
		"connection lost",
		"broken pipe",
		"econnreset",
		"econnrefused",
		"epipe",
		"socket hang up",
		"stream closed",
		"premature close",
		"provider 500",
		"provider 502",
		"provider 503",
		"provider 504",
	}
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func readPiJSONL(r io.Reader, out chan<- piRPCEnvelope, errs chan<- error, mirror io.Writer, activity *piActivityTracker) {
	defer close(out)
	// Always drain whatever Pi still has buffered on stdout before returning.
	// If we stop reading early (oversized line, decode error, mirror failure)
	// while Pi is mid-write, the OS pipe fills, Pi blocks on the write, and our
	// cmd.Wait() blocks on Pi — a deadlock. Discarding the remainder lets Pi
	// finish writing and exit so cmd.Wait() can return.
	defer func() { _, _ = io.Copy(io.Discard, r) }()

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

		if activity != nil {
			activity.Observe(raw)
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

func awaitPiInteractivePromptCompletion(ctx context.Context, stdin io.Writer, lines <-chan piRPCEnvelope, readErrs <-chan error, requestID string, obs *piRunObservability) error {
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
			if line.Type == "extension_ui_request" {
				if err := handlePiExtensionUIRequest(stdin, line.Raw); err != nil {
					return err
				}
				continue
			}
			if promptAccepted && line.Type == "agent_end" {
				return nil
			}
		}
	}
}

func handlePiExtensionUIRequest(stdin io.Writer, raw map[string]any) error {
	method, _ := raw["method"].(string)
	switch method {
	case "select", "confirm", "input", "editor":
		if !piIsInteractive() {
			return fmt.Errorf("pi requested interactive user input (%s) but Doug is not attached to an interactive terminal", method)
		}
		id, _ := raw["id"].(string)
		p := piNewPrompter()
		response := map[string]any{"type": "extension_ui_response", "id": id}
		switch method {
		case "select":
			options := piRawStringSlice(raw["options"])
			_, value, err := p.SelectOne(piDialogTitle(raw), options, 0)
			if err != nil {
				return fmt.Errorf("answer pi select request: %w", err)
			}
			response["value"] = value
		case "confirm":
			confirmed, err := p.Confirm(piDialogTitle(raw), false)
			if err != nil {
				return fmt.Errorf("answer pi confirm request: %w", err)
			}
			response["confirmed"] = confirmed
		case "input":
			value, err := p.Text(piDialogTitle(raw), "")
			if err != nil {
				return fmt.Errorf("answer pi input request: %w", err)
			}
			response["value"] = value
		case "editor":
			value, err := p.Compose(piDialogTitle(raw), piRawString(raw, "prefill"))
			if err != nil {
				return fmt.Errorf("answer pi editor request: %w", err)
			}
			response["value"] = value
		}
		if err := writePiJSONL(stdin, response); err != nil {
			return fmt.Errorf("send pi extension ui response: %w", err)
		}
	}
	return nil
}

func piDialogTitle(raw map[string]any) string {
	title := strings.TrimSpace(piRawString(raw, "title"))
	message := strings.TrimSpace(piRawString(raw, "message"))
	placeholder := strings.TrimSpace(piRawString(raw, "placeholder"))
	switch {
	case title != "" && message != "":
		return title + "\n" + message
	case title != "":
		if placeholder != "" {
			return title + "\n" + placeholder
		}
		return title
	case message != "":
		return message
	case placeholder != "":
		return placeholder
	default:
		return "Pi requested input"
	}
}

func piRawString(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

func piRawStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

type piActivityTracker struct {
	mu       sync.RWMutex
	observed bool
	label    string
}

func newPiActivityTracker() *piActivityTracker {
	return &piActivityTracker{label: "(no activity)"}
}

func (t *piActivityTracker) Observe(raw map[string]any) {
	if t == nil {
		return
	}
	label := ""
	if isPiToolCallEvent(raw) {
		label = formatPiToolActivity(raw)
	} else if isPiTextContentEvent(raw) {
		label = "generating..."
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.observed = true
	if label != "" {
		t.label = label
	}
}

func (t *piActivityTracker) String() string {
	if t == nil {
		return "(no activity)"
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.observed {
		return "(no activity)"
	}
	if t.label == "" {
		return "(no activity)"
	}
	return t.label
}

func formatPiToolActivity(raw map[string]any) string {
	name := truncateActivityPart(sanitizeActivityPart(firstPiString(raw, []string{"toolName", "tool_name", "name", "tool"})), 40)
	arg := truncateActivityPart(sanitizeActivityPart(firstPiString(raw, []string{"path", "file_path", "filepath", "command"})), 40)
	if name == "" {
		name = "tool"
	}
	if arg == "" {
		return name
	}
	return name + " " + arg
}

func isPiTextContentEvent(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if _, ok := m["text"].(string); ok {
		return true
	}
	t, _ := m["type"].(string)
	if strings.Contains(t, "content") || strings.Contains(t, "text") || strings.Contains(t, "message") {
		return true
	}
	for _, child := range m {
		if isPiTextContentEvent(child) {
			return true
		}
	}
	return false
}

func firstPiString(value any, keys []string) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	for _, child := range m {
		if s := firstPiString(child, keys); s != "" {
			return s
		}
	}
	return ""
}

func sanitizeActivityPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func truncateActivityPart(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

type piRunObservability struct {
	seenSessionIDs   map[string]struct{}
	orderedIDs       []string
	start            time.Time
	firstResponseFn  func(elapsed time.Duration)
	firstOnce        sync.Once
	firstSeen        bool
	firstResponse    time.Time
	toolCallCount    int
	providerFailures []types.ProviderFailure
}

func newPiRunObservability(start time.Time, firstResponseFn func(elapsed time.Duration)) *piRunObservability {
	return &piRunObservability{seenSessionIDs: make(map[string]struct{}), start: start, firstResponseFn: firstResponseFn}
}

func (o *piRunObservability) observe(line piRPCEnvelope) {
	if o == nil {
		return
	}
	if !isPiStartupLine(line) {
		o.recordFirstResponse(time.Now())
	}
	if isPiToolCallEvent(line.Raw) {
		o.toolCallCount++
	}
	o.providerFailures = append(o.providerFailures, extractPiProviderFailures(line.Raw)...)
	o.collect(line.Raw)
}

func (o *piRunObservability) recordFirstResponse(at time.Time) {
	o.firstOnce.Do(func() {
		o.firstSeen = true
		o.firstResponse = at
		if o.firstResponseFn != nil {
			o.firstResponseFn(at.Sub(o.start))
		}
	})
}

func (o *piRunObservability) firstResponseMs() int64 {
	if o == nil || !o.firstSeen {
		return 0
	}
	return o.firstResponse.Sub(o.start).Milliseconds()
}

func (o *piRunObservability) providerFailureDetails() []types.ProviderFailure {
	if o == nil || len(o.providerFailures) == 0 {
		return nil
	}
	out := make([]types.ProviderFailure, len(o.providerFailures))
	copy(out, o.providerFailures)
	return out
}

func isPiStartupLine(line piRPCEnvelope) bool {
	return line.Type == "response" && (line.ID == "doug-startup" || line.ID == "doug-prompt")
}

func isPiToolCallEvent(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	t, _ := m["type"].(string)
	if t == "tool_use" || t == "tool_call" || t == "tool_start" {
		return true
	}
	if _, ok := m["toolName"].(string); ok {
		return true
	}
	if _, ok := m["tool_name"].(string); ok {
		return true
	}
	return false
}

func extractPiProviderFailures(value any) []types.ProviderFailure {
	var out []types.ProviderFailure
	collectPiProviderFailures(value, &out)
	return out
}

func collectPiProviderFailures(value any, out *[]types.ProviderFailure) {
	switch v := value.(type) {
	case map[string]any:
		failureType, _ := v["type"].(string)
		if failureType == "provider_transport_failure" || failureType == "transport_failure" || failureType == "provider_failure" {
			failure := types.ProviderFailure{Type: failureType}
			failure.Message, _ = v["message"].(string)
			failure.Phase, _ = v["phase"].(string)
			*out = append(*out, failure)
		}
		for _, child := range v {
			collectPiProviderFailures(child, out)
		}
	case []any:
		for _, child := range v {
			collectPiProviderFailures(child, out)
		}
	}
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
