package agent

import (
	"context"
	"fmt"
	"path/filepath"
)

const piSessionRootDir = "pi-sessions"

// PiAdapter is a Doug-owned Backend implementation for future Pi RPC runs.
// Command handlers continue to speak only in terms of RunRequest/RunResponse;
// Pi-specific request preparation remains private to internal/agent.
type PiAdapter struct {
	launcher piLauncher
}

type piLauncher interface {
	Run(ctx context.Context, spec piLaunchSpec) (RunResponse, error)
}

type piLaunchSpec struct {
	WorkingDir string
	Request    piRPCRequest
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
	Read  piRPCRestrictionHook
	Write piRPCRestrictionHook
}

type piRPCRestrictionHook struct {
	Mode  string
	Paths []string
}

// NewPiAdapter constructs a Pi-backed backend boundary. The launcher is kept
// private so future Pi protocol work can evolve without changing call sites.
func NewPiAdapter() PiAdapter {
	return PiAdapter{launcher: rejectingPiLauncher{}}
}

// Run implements Backend by translating the Doug-native request into a
// Pi-owned launch spec, then delegating the Pi-specific work to a private
// launcher.
func (a PiAdapter) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	launcher := a.launcher
	if launcher == nil {
		launcher = rejectingPiLauncher{}
	}

	return launcher.Run(ctx, piLaunchSpec{
		WorkingDir: req.ProjectRoot,
		Request:    buildPiRPCRequest(req),
	})
}

func buildPiRPCRequest(req RunRequest) piRPCRequest {
	return piRPCRequest{
		Phase: phaseSessionComponent(req.Phase),
		Execution: piRPCExecution{
			Mode:    "one_shot",
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

type rejectingPiLauncher struct{}

func (rejectingPiLauncher) Run(_ context.Context, spec piLaunchSpec) (RunResponse, error) {
	return RunResponse{
		Status: RunStatusRejected,
	}, fmt.Errorf("pi adapter launcher is not configured")
}

func phaseSessionComponent(p RunPhase) string {
	if p == "" {
		return "runtime"
	}
	return string(p)
}
