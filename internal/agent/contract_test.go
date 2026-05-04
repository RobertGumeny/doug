package agent

import (
	"path/filepath"
	"strings"
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

func TestApplyPolicyScopeRestrictions(t *testing.T) {
	projectRoot := t.TempDir()
	dougDir := filepath.Join(projectRoot, ".doug")

	t.Run("no scopes leaves mode and paths unchanged", func(t *testing.T) {
		contract := RuntimeContract(projectRoot, dougDir)
		original := contract.Restrictions.Write.Mode
		originalLen := len(contract.Restrictions.Write.Paths)

		got := ApplyPolicyScopeRestrictions(contract, nil, nil)

		if got.Restrictions.Write.Mode != original {
			t.Fatalf("write mode = %q, want %q", got.Restrictions.Write.Mode, original)
		}
		if len(got.Restrictions.Write.Paths) != originalLen {
			t.Fatalf("write path count = %d, want %d", len(got.Restrictions.Write.Paths), originalLen)
		}
	})

	t.Run("write scopes upgrade mode to allow_list and append paths", func(t *testing.T) {
		contract := RuntimeContract(projectRoot, dougDir)
		originalPathCount := len(contract.Restrictions.Write.Paths)

		scopes := []string{"/extra/scope1", "/extra/scope2"}
		got := ApplyPolicyScopeRestrictions(contract, scopes, nil)

		if got.Restrictions.Write.Mode != RestrictionModeAllowList {
			t.Fatalf("write mode = %q, want %q", got.Restrictions.Write.Mode, RestrictionModeAllowList)
		}
		if len(got.Restrictions.Write.Paths) != originalPathCount+len(scopes) {
			t.Fatalf("write path count = %d, want %d", len(got.Restrictions.Write.Paths), originalPathCount+len(scopes))
		}
		if got.Restrictions.Write.Paths[len(got.Restrictions.Write.Paths)-1] != scopes[1] {
			t.Fatalf("last write path = %q, want %q", got.Restrictions.Write.Paths[len(got.Restrictions.Write.Paths)-1], scopes[1])
		}
	})

	t.Run("read additions appended without changing read mode", func(t *testing.T) {
		contract := RuntimeContract(projectRoot, dougDir)
		originalMode := contract.Restrictions.Read.Mode
		originalPathCount := len(contract.Restrictions.Read.Paths)

		additions := []string{"/extra/docs"}
		got := ApplyPolicyScopeRestrictions(contract, nil, additions)

		if got.Restrictions.Read.Mode != originalMode {
			t.Fatalf("read mode = %q, want %q", got.Restrictions.Read.Mode, originalMode)
		}
		if len(got.Restrictions.Read.Paths) != originalPathCount+1 {
			t.Fatalf("read path count = %d, want %d", len(got.Restrictions.Read.Paths), originalPathCount+1)
		}
		if got.Restrictions.Read.Paths[len(got.Restrictions.Read.Paths)-1] != additions[0] {
			t.Fatalf("last read path = %q, want %q", got.Restrictions.Read.Paths[len(got.Restrictions.Read.Paths)-1], additions[0])
		}
	})

	t.Run("write scopes on already-allow-list contract keep mode and append paths", func(t *testing.T) {
		planPath := filepath.Join(dougDir, "plan", "PLAN.md")
		contract := PlanningContract(projectRoot, dougDir, planPath)
		originalPathCount := len(contract.Restrictions.Write.Paths)

		scopes := []string{"/extra/scope"}
		got := ApplyPolicyScopeRestrictions(contract, scopes, nil)

		if got.Restrictions.Write.Mode != RestrictionModeAllowList {
			t.Fatalf("write mode = %q, want allow_list", got.Restrictions.Write.Mode)
		}
		if len(got.Restrictions.Write.Paths) != originalPathCount+1 {
			t.Fatalf("write path count = %d, want %d", len(got.Restrictions.Write.Paths), originalPathCount+1)
		}
	})

	t.Run("does not mutate the original contract", func(t *testing.T) {
		contract := RuntimeContract(projectRoot, dougDir)
		originalWriteMode := contract.Restrictions.Write.Mode
		originalWriteLen := len(contract.Restrictions.Write.Paths)

		_ = ApplyPolicyScopeRestrictions(contract, []string{"/extra"}, []string{"/read-extra"})

		if contract.Restrictions.Write.Mode != originalWriteMode {
			t.Fatal("ApplyPolicyScopeRestrictions mutated the original contract write mode")
		}
		if len(contract.Restrictions.Write.Paths) != originalWriteLen {
			t.Fatal("ApplyPolicyScopeRestrictions mutated the original contract write paths")
		}
	})
}

func TestWriteScopeSection(t *testing.T) {
	t.Run("returns nil when no scopes", func(t *testing.T) {
		if got := WriteScopeSection(nil); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
		if got := WriteScopeSection([]string{}); got != nil {
			t.Fatalf("expected nil for empty slice, got %+v", got)
		}
	})

	t.Run("returns section with heading and all paths listed", func(t *testing.T) {
		scopes := []string{"/path/a", "/path/b"}
		got := WriteScopeSection(scopes)
		if got == nil {
			t.Fatal("expected non-nil section")
		}
		if got.Heading != "Write Scope Constraints" {
			t.Fatalf("heading = %q, want %q", got.Heading, "Write Scope Constraints")
		}
		for _, path := range scopes {
			if !strings.Contains(got.Body, path) {
				t.Errorf("body missing path %q", path)
			}
		}
	})
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
