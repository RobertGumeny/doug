package agent

import "path/filepath"

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
func ScaffoldContract(projectRoot, dougDir string) RunContract {
	return RuntimeContract(projectRoot, dougDir)
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
			Read:  RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{agentsPath, prdPath, activeTaskPath, planPath}},
			Write: RestrictionHook{Mode: RestrictionModeAllowList, Paths: []string{activeTaskPath, planPath}},
		},
	}
}

// PostEpicKBContract returns the default contract for the post-epic KB pass.
func PostEpicKBContract(projectRoot, dougDir, epicID string) RunContract {
	activeTaskPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
	prdPath := filepath.Join(dougDir, "PRD.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
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
		},
		Artifacts: ArtifactSurfaces{
			Read: []ArtifactSurface{
				{Path: agentsPath, Purpose: ArtifactPurposeProjectInstructions, Authority: ArtifactAuthorityProject, AgentFacing: true},
				{Path: prdPath, Purpose: ArtifactPurposeProductContext, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				{Path: activeTaskPath, Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
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
			Read:  RestrictionHook{Mode: RestrictionModeInherit, Paths: []string{agentsPath, prdPath, activeTaskPath, kbRoot, runtimeArchive, sessionArchive}},
			Write: RestrictionHook{Mode: RestrictionModeAllowList, Paths: []string{kbRoot, activeTaskPath}},
		},
	}
}
