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
	if len(contract.Artifacts.Write) != 2 {
		t.Fatalf("write artifact count = %d, want 2", len(contract.Artifacts.Write))
	}
	if contract.Artifacts.Write[1].Purpose != ArtifactPurposeWorkingArtifact {
		t.Fatalf("unexpected write artifact: %+v", contract.Artifacts.Write[1])
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
