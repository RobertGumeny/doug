package agent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RunContract centralizes the Doug-native artifact contract for one workflow.
type RunContract struct {
	Brief            CanonicalBrief
	ContextLoadOrder []ContextInput
	Artifacts        ArtifactSurfaces
	Restrictions     RestrictionHooks
}

// RuntimeContract returns the default contract for doug runtime task execution.
func RuntimeContract(projectRoot, dougDir string) RunContract {
	activeTaskPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
	prdPath := filepath.Join(dougDir, "PRD.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	activeBugPath := filepath.Join(dougDir, "ACTIVE_BUG.md")
	activeFailurePath := filepath.Join(dougDir, "ACTIVE_FAILURE.md")

	return RunContract{
		Brief: CanonicalBrief{
			Path:      activeTaskPath,
			Format:    BriefFormatMarkdown,
			Authority: ArtifactAuthorityDoug,
		},
		ContextLoadOrder: []ContextInput{
			{Kind: ContextInputProjectInstructions, Path: agentsPath, Required: false, Authority: ArtifactAuthorityProject},
			{Kind: ContextInputProductContext, Path: prdPath, Required: false, Authority: ArtifactAuthorityDoug},
			{Kind: ContextInputCanonicalBrief, Path: activeTaskPath, Required: true, Authority: ArtifactAuthorityDoug},
		},
		Artifacts: ArtifactSurfaces{
			Read: []ArtifactSurface{
				{Path: projectRoot, Purpose: ArtifactPurposeProjectWorkspace, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: agentsPath, Purpose: ArtifactPurposeProjectInstructions, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: prdPath, Purpose: ArtifactPurposeProductContext, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
			},
			Write: []ArtifactSurface{
				{Path: projectRoot, Purpose: ArtifactPurposeProjectWorkspace, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeBugPath, Purpose: ArtifactPurposeBugHandoff, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeFailurePath, Purpose: ArtifactPurposeFailureHandoff, Authority: ArtifactAuthorityDoug, AgentFacing: false},
			},
		},
		Restrictions: RestrictionHooks{
			Read:  RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{projectRoot, agentsPath, prdPath, activeTaskPath}},
			Write: RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{projectRoot, activeTaskPath, activeBugPath, activeFailurePath}},
		},
	}
}

// ScaffoldContract returns the default contract for scaffold runs.
func ScaffoldContract(projectRoot, dougDir, manifestPath string) RunContract {
	runtime := RuntimeContract(projectRoot, dougDir)
	runtime.ContextLoadOrder = append(runtime.ContextLoadOrder, ContextInput{
		Kind:      ContextInputWorkingArtifact,
		Path:      manifestPath,
		Required:  true,
		Authority: ArtifactAuthorityDoug,
	})
	runtime.Artifacts.Read = append(runtime.Artifacts.Read, ArtifactSurface{
		Path:        manifestPath,
		Purpose:     ArtifactPurposeWorkingArtifact,
		Authority:   ArtifactAuthorityDoug,
		AgentFacing: true,
	})
	runtime.Restrictions.Read.Paths = append(runtime.Restrictions.Read.Paths, manifestPath)
	return runtime
}

// PlanningContract returns the default contract for planning runs.
func PlanningContract(projectRoot, dougDir, planPath string) RunContract {
	activeTaskPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
	prdPath := filepath.Join(dougDir, "PRD.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")

	return RunContract{
		Brief: CanonicalBrief{
			Path:      activeTaskPath,
			Format:    BriefFormatMarkdown,
			Authority: ArtifactAuthorityDoug,
		},
		ContextLoadOrder: []ContextInput{
			{Kind: ContextInputProjectInstructions, Path: agentsPath, Required: false, Authority: ArtifactAuthorityProject},
			{Kind: ContextInputProductContext, Path: prdPath, Required: false, Authority: ArtifactAuthorityDoug},
			{Kind: ContextInputCanonicalBrief, Path: activeTaskPath, Required: true, Authority: ArtifactAuthorityDoug},
			{Kind: ContextInputWorkingArtifact, Path: planPath, Required: true, Authority: ArtifactAuthorityDoug},
		},
		Artifacts: ArtifactSurfaces{
			Read: []ArtifactSurface{
				{Path: projectRoot, Purpose: ArtifactPurposeProjectWorkspace, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: agentsPath, Purpose: ArtifactPurposeProjectInstructions, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: prdPath, Purpose: ArtifactPurposeProductContext, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: planPath, Purpose: ArtifactPurposeWorkingArtifact, Authority: ArtifactAuthorityDoug, AgentFacing: true},
			},
			Write: []ArtifactSurface{
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: planPath, Purpose: ArtifactPurposeWorkingArtifact, Authority: ArtifactAuthorityDoug, AgentFacing: true},
			},
		},
		Restrictions: RestrictionHooks{
			Read:  RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{projectRoot, agentsPath, prdPath, activeTaskPath, planPath}},
			Write: RestrictionHook{Mode: RestrictionModeAllowList, Paths: []string{activeTaskPath, planPath}},
		},
	}
}

// ApplyPolicyScopeRestrictions merges policy-resolved write scopes and read path
// additions into the contract's restrictions. When write scopes are non-empty, the
// write restriction mode is upgraded to AllowList so Pi can enforce the boundary
// natively. Read path additions are appended to the read restriction path list;
// the read mode stays Inherit to avoid over-restricting agent access to project files.
func ApplyPolicyScopeRestrictions(contract RunContract, writeScopes, readAdditions []string) RunContract {
	if len(readAdditions) > 0 {
		contract.Restrictions.Read.Paths = append(contract.Restrictions.Read.Paths, readAdditions...)
	}
	if len(writeScopes) > 0 {
		contract.Restrictions.Write.Paths = append(contract.Restrictions.Write.Paths, writeScopes...)
		contract.Restrictions.Write.Mode = RestrictionModeAllowList
	}
	return contract
}

// WriteScopeSection returns a structured ActiveTaskSection documenting additional
// write scope constraints configured for the current run. Returns nil when
// writeScopes is empty. The section is injected into ACTIVE_TASK.md so the
// policy restriction contract is explicit to the agent as well as Pi.
func WriteScopeSection(writeScopes []string) *ActiveTaskSection {
	if len(writeScopes) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("This task is configured with additional write scope constraints. Writes are permitted only to the task workspace and the following explicitly allowed paths:\n")
	for _, path := range writeScopes {
		fmt.Fprintf(&sb, "- %s\n", path)
	}
	sb.WriteString("\nDo not write outside these paths or the project task workspace.")
	return &ActiveTaskSection{
		Heading: "Write Scope Constraints",
		Body:    sb.String(),
	}
}

// ResearchContract returns the default contract for research runs.
// Research reads the full project workspace but writes only to
// .doug/logs/research/ and .doug/ACTIVE_TASK.md — no project-root artifacts.
func ResearchContract(projectRoot, dougDir string) RunContract {
	activeTaskPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
	prdPath := filepath.Join(dougDir, "PRD.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	researchLogsPath := filepath.Join(dougDir, "logs", "research")

	return RunContract{
		Brief: CanonicalBrief{
			Path:      activeTaskPath,
			Format:    BriefFormatMarkdown,
			Authority: ArtifactAuthorityDoug,
		},
		ContextLoadOrder: []ContextInput{
			{Kind: ContextInputProjectInstructions, Path: agentsPath, Required: false, Authority: ArtifactAuthorityProject},
			{Kind: ContextInputProductContext, Path: prdPath, Required: false, Authority: ArtifactAuthorityDoug},
			{Kind: ContextInputCanonicalBrief, Path: activeTaskPath, Required: true, Authority: ArtifactAuthorityDoug},
		},
		Artifacts: ArtifactSurfaces{
			Read: []ArtifactSurface{
				{Path: projectRoot, Purpose: ArtifactPurposeProjectWorkspace, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: agentsPath, Purpose: ArtifactPurposeProjectInstructions, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: prdPath, Purpose: ArtifactPurposeProductContext, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
			},
			Write: []ArtifactSurface{
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: researchLogsPath, Purpose: ArtifactPurposeWorkingArtifact, Authority: ArtifactAuthorityDoug, AgentFacing: true},
			},
		},
		Restrictions: RestrictionHooks{
			Read:  RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{projectRoot, agentsPath, prdPath, activeTaskPath}},
			Write: RestrictionHook{Mode: RestrictionModeAllowList, Paths: []string{activeTaskPath, researchLogsPath}},
		},
	}
}

// PostEpicKBContract returns the default contract for the post-epic KB pass.
func PostEpicKBContract(projectRoot, dougDir, epicID string) RunContract {
	activeTaskPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
	prdPath := filepath.Join(dougDir, "PRD.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	planPath := filepath.Join(dougDir, "plan", "PLAN.md")
	kbRoot := filepath.Join(projectRoot, "docs", "kb")
	runtimeArchive := filepath.Join(dougDir, "logs", "archives", epicID)
	sessionArchive := filepath.Join(dougDir, "logs", "sessions", epicID)

	return RunContract{
		Brief: CanonicalBrief{
			Path:      activeTaskPath,
			Format:    BriefFormatMarkdown,
			Authority: ArtifactAuthorityDoug,
		},
		ContextLoadOrder: []ContextInput{
			{Kind: ContextInputProjectInstructions, Path: agentsPath, Required: false, Authority: ArtifactAuthorityProject},
			{Kind: ContextInputProductContext, Path: prdPath, Required: false, Authority: ArtifactAuthorityDoug},
			{Kind: ContextInputCanonicalBrief, Path: activeTaskPath, Required: true, Authority: ArtifactAuthorityDoug},
			{Kind: ContextInputWorkingArtifact, Path: planPath, Required: false, Authority: ArtifactAuthorityDoug},
		},
		Artifacts: ArtifactSurfaces{
			Read: []ArtifactSurface{
				{Path: agentsPath, Purpose: ArtifactPurposeProjectInstructions, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: prdPath, Purpose: ArtifactPurposeProductContext, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: planPath, Purpose: ArtifactPurposeWorkingArtifact, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: kbRoot, Purpose: ArtifactPurposeKnowledgeBase, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: runtimeArchive, Purpose: ArtifactPurposeRuntimeArchive, Authority: ArtifactAuthorityDoug, AgentFacing: false},
				{Path: sessionArchive, Purpose: ArtifactPurposeSessionArchive, Authority: ArtifactAuthorityDoug, AgentFacing: false},
			},
			Write: []ArtifactSurface{
				{Path: kbRoot, Purpose: ArtifactPurposeKnowledgeBase, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
			},
		},
		Restrictions: RestrictionHooks{
			Read:  RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{agentsPath, prdPath, activeTaskPath, planPath, kbRoot, runtimeArchive, sessionArchive}},
			Write: RestrictionHook{Mode: RestrictionModeAllowList, Paths: []string{kbRoot, activeTaskPath}},
		},
	}
}
