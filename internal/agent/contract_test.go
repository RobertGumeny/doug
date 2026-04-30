package agent

import (
	"path/filepath"
	"testing"
)

func TestPlanningContract(t *testing.T) {
	projectRoot := t.TempDir()
	dougDir := filepath.Join(projectRoot, ".doug")
	planPath := filepath.Join(dougDir, "plan", "PLAN.md")

	contract := PlanningContract(projectRoot, dougDir, planPath)

	if contract.Brief.Authority != ArtifactAuthorityDoug {
		t.Fatalf("brief authority = %q, want %q", contract.Brief.Authority, ArtifactAuthorityDoug)
	}
	if len(contract.ContextLoadOrder) != 4 {
		t.Fatalf("contextLoadOrder length = %d, want 4", len(contract.ContextLoadOrder))
	}
	if got := contract.ContextLoadOrder[0].Authority; got != ArtifactAuthorityProject {
		t.Fatalf("project instructions authority = %q, want %q", got, ArtifactAuthorityProject)
	}
	if got := contract.ContextLoadOrder[3].Authority; got != ArtifactAuthorityDoug {
		t.Fatalf("working artifact authority = %q, want %q", got, ArtifactAuthorityDoug)
	}
	if len(contract.Artifacts.Read) != 5 {
		t.Fatalf("read artifact count = %d, want 5", len(contract.Artifacts.Read))
	}
	if contract.Artifacts.Read[0].Path != projectRoot || contract.Artifacts.Read[0].Purpose != ArtifactPurposeProjectWorkspace {
		t.Fatalf("unexpected project workspace read artifact: %+v", contract.Artifacts.Read[0])
	}
	if len(contract.Artifacts.Write) != 2 {
		t.Fatalf("write artifact count = %d, want 2", len(contract.Artifacts.Write))
	}
	if contract.Artifacts.Write[1].Purpose != ArtifactPurposeWorkingArtifact {
		t.Fatalf("unexpected write artifact: %+v", contract.Artifacts.Write[1])
	}
	if len(contract.Restrictions.Read.Paths) != 5 || contract.Restrictions.Read.Paths[0] != projectRoot {
		t.Fatalf("unexpected read restriction paths: %+v", contract.Restrictions.Read.Paths)
	}
}

func TestScaffoldContract(t *testing.T) {
	projectRoot := t.TempDir()
	dougDir := filepath.Join(projectRoot, ".doug")
	manifestPath := filepath.Join(dougDir, "plan", "manifest.yaml")

	contract := ScaffoldContract(projectRoot, dougDir, manifestPath)

	if len(contract.ContextLoadOrder) != 4 {
		t.Fatalf("contextLoadOrder length = %d, want 4", len(contract.ContextLoadOrder))
	}
	if got := contract.ContextLoadOrder[3]; got.Kind != ContextInputWorkingArtifact || got.Path != manifestPath || !got.Required || got.Authority != ArtifactAuthorityDoug {
		t.Fatalf("unexpected manifest working artifact context: %+v", got)
	}
	if len(contract.Artifacts.Read) != 5 {
		t.Fatalf("read artifact count = %d, want 5", len(contract.Artifacts.Read))
	}
	if got := contract.Artifacts.Read[4]; got.Path != manifestPath || got.Purpose != ArtifactPurposeWorkingArtifact || got.Authority != ArtifactAuthorityDoug || !got.AgentFacing {
		t.Fatalf("unexpected manifest read artifact: %+v", got)
	}
	if len(contract.Artifacts.Write) != 4 {
		t.Fatalf("write artifact count = %d, want 4", len(contract.Artifacts.Write))
	}
	if len(contract.Restrictions.Read.Paths) != 5 || contract.Restrictions.Read.Paths[4] != manifestPath {
		t.Fatalf("unexpected read restriction paths: %+v", contract.Restrictions.Read.Paths)
	}
}

func TestPostEpicKBContract(t *testing.T) {
	projectRoot := t.TempDir()
	dougDir := filepath.Join(projectRoot, ".doug")

	contract := PostEpicKBContract(projectRoot, dougDir, "EPIC-9")

	if len(contract.Artifacts.Read) != 6 {
		t.Fatalf("read artifact count = %d, want 6", len(contract.Artifacts.Read))
	}
	if contract.Artifacts.Read[4].Purpose != ArtifactPurposeRuntimeArchive {
		t.Fatalf("unexpected runtime archive artifact: %+v", contract.Artifacts.Read[4])
	}
	if contract.Artifacts.Read[4].AgentFacing {
		t.Fatalf("runtime archive should not be agent-facing: %+v", contract.Artifacts.Read[4])
	}
	if contract.Restrictions.Write.Mode != RestrictionModeAllowList {
		t.Fatalf("write restriction mode = %q, want %q", contract.Restrictions.Write.Mode, RestrictionModeAllowList)
	}
}
