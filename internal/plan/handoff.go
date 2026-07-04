package plan

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

const (
	planSchemaVersionV1 = 1
	planFileName        = "PLAN.md"
	handoffSectionTitle = "## Handoff Data"

	// placeholderPRDSentence is the sentinel phrase embedded in the seeded PRD
	// block. Its presence indicates the prd field has not been authored yet.
	placeholderPRDSentence = "Describe the epic's product requirements here."
)

var handoffDataPattern = regexp.MustCompile("(?ms)^## Handoff Data[^\\n]*\\r?\\n.*?^```yaml[ \\t]*\\r?\\n(.*?)\\r?\\n^```\\s*(?:\\r?\\n|$)")

// epicDirPattern matches a concrete backlog epic directory name (EPIC-<N>) so
// handoff can derive the highest already-allocated epic number.
var epicDirPattern = regexp.MustCompile(`^EPIC-(\d+)$`)

// submittedEpicIDPattern matches a well-formed submitted epic identifier: a
// concrete EPIC-<N> or a placeholder token such as EPIC-<X>. It is enforced
// before normalization so malformed identifiers are rejected before any
// backlog package is written.
var submittedEpicIDPattern = regexp.MustCompile(`^EPIC-(?:\d+|<[A-Za-z0-9_]+>)$`)

// knownPlaceholders contains the exact seed-template strings written by
// initialPlanDocument for free-form text fields. These are rejected when they
// survive into handoff-ready PLAN.md content.
var knownPlaceholders = map[string]bool{
	"My Project":                   true,
	"Example Epic":                 true,
	"Describe the task here.":      true,
	"First acceptance criterion.":  true,
	"Second acceptance criterion.": true,
}

// isPlaceholder reports whether s (after trimming whitespace) is a known seed
// placeholder value that must not appear in a handoff-ready PLAN.md.
func isPlaceholder(s string) bool {
	return knownPlaceholders[strings.TrimSpace(s)]
}

// HandoffResult summarizes the deterministic outputs generated from PLAN.md.
type HandoffResult struct {
	EpicCount         int
	ManifestGenerated bool
	ArchivedPlanPath  string
	// CoercedBugfixTaskIDs lists task IDs authored as runtime-only type
	// "bugfix" that handoff rewrote to "feature". bugfix is scheduled only by
	// the run loop's self-heal flow; coercing here keeps a bugfix-flavored plan
	// from either failing handoff or deadlocking the run loop at dispatch.
	CoercedBugfixTaskIDs []string
}

type HandoffDocument struct {
	SchemaVersion int             `yaml:"schema_version"`
	Project       HandoffProject  `yaml:"project"`
	Manifest      *types.Manifest `yaml:"manifest,omitempty"`
	Epics         []HandoffEpic   `yaml:"epics"`

	// coercedBugfixTaskIDs accumulates task IDs rewritten from authored type
	// "bugfix" to "feature" during validation. Unexported so YAML never
	// serializes it; surfaced to callers via HandoffResult.
	coercedBugfixTaskIDs []string
}

type HandoffProject struct {
	Name string `yaml:"name"`
	Mode string `yaml:"mode"`
}

type HandoffEpic struct {
	ID    string        `yaml:"id"`
	Name  string        `yaml:"name"`
	PRD   string        `yaml:"prd"`
	Tasks []HandoffTask `yaml:"tasks"`
}

type HandoffTask struct {
	ID                 string         `yaml:"id"`
	Type               types.TaskType `yaml:"type,omitempty"`
	Status             types.Status   `yaml:"status,omitempty"`
	Description        string         `yaml:"description"`
	AcceptanceCriteria []string       `yaml:"acceptance_criteria"`
}

// ParseHandoffDocument reads a structured PLAN.md and extracts the YAML payload
// from the required "## Handoff Data" section.
func ParseHandoffDocument(path string) (*HandoffDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read PLAN.md %q: %w", path, err)
	}

	return parseHandoffDocumentData(path, data)
}

func parseHandoffDocumentData(path string, data []byte) (*HandoffDocument, error) {

	match := handoffDataPattern.FindSubmatch(data)
	if len(match) != 2 {
		return nil, fmt.Errorf("parse PLAN.md %q: missing %q fenced yaml block", path, handoffSectionTitle)
	}

	var doc HandoffDocument
	dec := yaml.NewDecoder(bytes.NewReader(match[1]))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse PLAN.md %q handoff data: %w", path, err)
	}

	if err := validateHandoffDocument(path, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// HandoffProjectPlan parses PLAN.md and emits deterministic backlog epic
// packages plus manifest.yaml when greenfield scaffold data is present.
func HandoffProjectPlan(projectRoot string, now time.Time) (*HandoffResult, error) {
	planPath := filepath.Join(projectRoot, ".doug", backlogPlanDirName, planFileName)
	planData, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("read PLAN.md %q: %w", planPath, err)
	}

	doc, err := parseHandoffDocumentData(planPath, planData)
	if err != nil {
		return nil, err
	}

	if err := validateSubmittedIDs(planPath, doc); err != nil {
		return nil, err
	}

	if err := normalizeEpicIDs(projectRoot, doc); err != nil {
		return nil, err
	}

	timestamp := now.UTC().Format(time.RFC3339)
	result := &HandoffResult{
		EpicCount:            len(doc.Epics),
		CoercedBugfixTaskIDs: doc.coercedBugfixTaskIDs,
	}
	for _, epic := range doc.Epics {
		if err := writeEpicPackage(projectRoot, timestamp, epic); err != nil {
			return nil, err
		}
	}

	if doc.Manifest != nil {
		manifestPath := filepath.Join(projectRoot, ".doug", backlogPlanDirName, "manifest.yaml")
		if err := writeManifest(manifestPath, doc.Manifest); err != nil {
			return nil, err
		}
		result.ManifestGenerated = true
	}

	archivePath, err := archivePlanWorkbook(projectRoot, now, planData)
	if err != nil {
		return nil, err
	}
	result.ArchivedPlanPath = archivePath

	nextWorkbook := InitialPlanDocument(WorkbookContext{
		LastHandoffAt:      timestamp,
		LastHandoffArchive: archivePath,
		LastHandoffEpicIDs: handoffEpicIDs(doc.Epics),
	})
	if err := state.AtomicWrite(planPath, []byte(nextWorkbook)); err != nil {
		restoreErr := state.AtomicWrite(planPath, planData)
		if restoreErr != nil {
			return nil, fmt.Errorf("reset PLAN.md after handoff: %v (restore failed: %v)", err, restoreErr)
		}
		return nil, fmt.Errorf("reset PLAN.md after handoff: %w", err)
	}

	return result, nil
}

func archivePlanWorkbook(projectRoot string, now time.Time, data []byte) (string, error) {
	historyDir := filepath.Join(projectRoot, ".doug", backlogPlanDirName, "history")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		return "", fmt.Errorf("create plan history directory %q: %w", historyDir, err)
	}

	archiveName := fmt.Sprintf("PLAN-%s.md", now.UTC().Format("20060102T150405.000000000Z"))
	archivePath := filepath.Join(historyDir, archiveName)
	if err := state.AtomicWrite(archivePath, data); err != nil {
		return "", fmt.Errorf("archive PLAN.md to %q: %w", archivePath, err)
	}
	return filepath.ToSlash(filepath.Join(".doug", backlogPlanDirName, "history", archiveName)), nil
}

func handoffEpicIDs(epics []HandoffEpic) []string {
	ids := make([]string, 0, len(epics))
	for _, epic := range epics {
		ids = append(ids, epic.ID)
	}
	return ids
}

// RenderTasksYAML writes tasks.yaml deterministically and forces double quotes
// around parser-sensitive string fields.
func RenderTasksYAML(tasks *types.Tasks) ([]byte, error) {
	if tasks == nil {
		return nil, fmt.Errorf("render tasks.yaml: tasks is nil")
	}

	doc := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			mappingNode(
				scalarNode("epic"), mappingNode(
					scalarNode("id"), quotedScalar(tasks.Epic.ID),
					scalarNode("name"), quotedScalar(tasks.Epic.Name),
					scalarNode("tasks"), taskSequenceNode(tasks.Epic.Tasks),
				),
			),
		},
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("render tasks.yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("render tasks.yaml: %w", err)
	}
	return buf.Bytes(), nil
}

func validateHandoffDocument(path string, doc *HandoffDocument) error {
	if doc == nil {
		return fmt.Errorf("invalid PLAN.md %q: handoff data is nil", path)
	}
	if doc.SchemaVersion != planSchemaVersionV1 {
		return fmt.Errorf(
			"invalid PLAN.md %q: unsupported schema_version %d (supported: %d)",
			path,
			doc.SchemaVersion,
			planSchemaVersionV1,
		)
	}
	if strings.TrimSpace(doc.Project.Name) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, "project.name")
	}
	if isPlaceholder(doc.Project.Name) {
		return fmt.Errorf("invalid PLAN.md %q: project.name %q is a seed placeholder; replace it with the actual project name", path, doc.Project.Name)
	}
	if strings.TrimSpace(doc.Project.Mode) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, "project.mode")
	}
	if len(doc.Epics) == 0 {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, "epics")
	}

	seenEpics := make(map[string]struct{}, len(doc.Epics))
	for i := range doc.Epics {
		if err := validateHandoffEpic(path, i, &doc.Epics[i], seenEpics, &doc.coercedBugfixTaskIDs); err != nil {
			return err
		}
	}

	if doc.Manifest != nil {
		if err := types.ValidateManifest(path, doc.Manifest); err != nil {
			return err
		}
	}
	return nil
}

func validateHandoffEpic(path string, index int, epic *HandoffEpic, seen map[string]struct{}, coerced *[]string) error {
	fieldPrefix := fmt.Sprintf("epics[%d]", index)
	if strings.TrimSpace(epic.ID) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".id")
	}
	if strings.TrimSpace(epic.Name) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".name")
	}
	if isPlaceholder(epic.Name) {
		return fmt.Errorf("invalid PLAN.md %q: %s.name %q is a seed placeholder; replace it with the actual epic name", path, fieldPrefix, epic.Name)
	}
	if strings.TrimSpace(epic.PRD) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".prd")
	}
	if strings.Contains(epic.PRD, placeholderPRDSentence) {
		return fmt.Errorf("invalid PLAN.md %q: %s.prd contains seed placeholder text; replace it with the actual epic product requirements", path, fieldPrefix)
	}
	if len(epic.Tasks) == 0 {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".tasks")
	}
	if _, ok := seen[epic.ID]; ok {
		return fmt.Errorf("invalid PLAN.md %q: duplicate epic id %q", path, epic.ID)
	}
	seen[epic.ID] = struct{}{}

	seenTasks := make(map[string]struct{}, len(epic.Tasks))
	for i := range epic.Tasks {
		if err := validateHandoffTask(path, fieldPrefix, i, &epic.Tasks[i], seenTasks, coerced); err != nil {
			return err
		}
	}
	return nil
}

func validateHandoffTask(path, epicPrefix string, index int, task *HandoffTask, seen map[string]struct{}, coerced *[]string) error {
	fieldPrefix := fmt.Sprintf("%s.tasks[%d]", epicPrefix, index)
	if strings.TrimSpace(task.ID) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".id")
	}
	if strings.TrimSpace(task.Description) == "" {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".description")
	}
	if isPlaceholder(task.Description) {
		return fmt.Errorf("invalid PLAN.md %q: %s.description %q is a seed placeholder; replace it with the actual task description", path, fieldPrefix, task.Description)
	}
	if len(task.AcceptanceCriteria) == 0 {
		return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fieldPrefix+".acceptance_criteria")
	}
	for i, criterion := range task.AcceptanceCriteria {
		if strings.TrimSpace(criterion) == "" {
			return fmt.Errorf("invalid PLAN.md %q: missing required field %q", path, fmt.Sprintf("%s.acceptance_criteria[%d]", fieldPrefix, i))
		}
		if isPlaceholder(criterion) {
			return fmt.Errorf("invalid PLAN.md %q: %s.acceptance_criteria[%d] %q is a seed placeholder; replace it with a real acceptance criterion", path, fieldPrefix, i, criterion)
		}
	}
	if _, ok := seen[task.ID]; ok {
		return fmt.Errorf("invalid PLAN.md %q: duplicate task id %q", path, task.ID)
	}
	seen[task.ID] = struct{}{}

	if task.Type == "" {
		task.Type = types.TaskTypeFeature
	}
	if task.Status == "" {
		task.Status = types.StatusTODO
	}
	switch task.Type {
	case types.TaskTypeFeature, types.TaskTypeDocumentation:
		// valid user-authored task types
	case types.TaskTypeBugfix:
		// bugfix is a runtime-only synthetic type: a dispatchable bugfix must
		// carry a Doug-scheduled BUG-<id> and bug payload (see
		// orchestrator.run dispatch guard), which an authored task never has.
		// Rather than reject a bugfix-flavored plan (or let it deadlock the run
		// loop at dispatch), coerce it to feature — the acceptance criteria
		// already describe the fix — and record the rewrite for a warning.
		task.Type = types.TaskTypeFeature
		if coerced != nil {
			*coerced = append(*coerced, task.ID)
		}
	default:
		return fmt.Errorf("invalid PLAN.md %q: unsupported task type %q in %s (supported: feature, documentation)", path, task.Type, fieldPrefix)
	}
	switch task.Status {
	case types.StatusTODO, types.StatusInProgress, types.StatusDone, types.StatusBlocked:
	default:
		return fmt.Errorf("invalid PLAN.md %q: unsupported task status %q in %s", path, task.Status, fieldPrefix)
	}
	return nil
}

// epicIDMapping records the rewrite from a submitted epic identifier (whether a
// placeholder token such as EPIC-<X> or a concrete EPIC-N) to the concrete
// identifier allocated at handoff.
type epicIDMapping struct {
	oldID string
	newID string
}

// validateSubmittedIDs enforces the shape of submitted epic and task
// identifiers and their internal references before normalization. Every epic
// ID must be a concrete EPIC-<N> or a placeholder token (EPIC-<X>), and every
// task ID must reuse its epic's submitted ID as a prefix followed by a numeric
// suffix. Rejection happens before any backlog package is written.
func validateSubmittedIDs(path string, doc *HandoffDocument) error {
	for i := range doc.Epics {
		epic := &doc.Epics[i]
		fieldPrefix := fmt.Sprintf("epics[%d]", i)
		if !submittedEpicIDPattern.MatchString(epic.ID) {
			return fmt.Errorf("invalid PLAN.md %q: %s.id %q is malformed; expected a concrete EPIC-<N> or a placeholder like EPIC-<X>", path, fieldPrefix, epic.ID)
		}
		expectedPrefix := epic.ID + "-"
		for j := range epic.Tasks {
			taskID := epic.Tasks[j].ID
			taskField := fmt.Sprintf("%s.tasks[%d]", fieldPrefix, j)
			if !strings.HasPrefix(taskID, expectedPrefix) {
				return fmt.Errorf("invalid PLAN.md %q: %s.id %q does not use its epic's submitted prefix %q", path, taskField, taskID, epic.ID)
			}
			suffix := taskID[len(expectedPrefix):]
			if !isAllDigits(suffix) {
				return fmt.Errorf("invalid PLAN.md %q: %s.id %q is malformed; expected %s<NNN> with a numeric suffix", path, taskField, taskID, expectedPrefix)
			}
		}
	}
	return nil
}

// isAllDigits reports whether s is non-empty and contains only ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// normalizeEpicIDs allocates concrete, gap-free epic identifiers for every
// submitted epic in document order, then rewrites all exact references to the
// submitted epic/task identifiers (in epic/task IDs, prd, task descriptions,
// and acceptance criteria) to the allocated identifiers. Allocation always uses
// the highest existing numeric EPIC-N plus one and never fills numeric gaps.
func normalizeEpicIDs(projectRoot string, doc *HandoffDocument) error {
	start, err := maxExistingEpicNumber(projectRoot)
	if err != nil {
		return err
	}

	mappings := make([]epicIDMapping, 0, len(doc.Epics))
	for i := range doc.Epics {
		mappings = append(mappings, epicIDMapping{
			oldID: doc.Epics[i].ID,
			newID: fmt.Sprintf("EPIC-%d", start+1+i),
		})
	}

	// Rewrite the identifier fields first.
	for i := range doc.Epics {
		epic := &doc.Epics[i]
		oldEpic := mappings[i].oldID
		newEpic := mappings[i].newID
		for j := range epic.Tasks {
			epic.Tasks[j].ID = normalizeTaskID(epic.Tasks[j].ID, oldEpic, newEpic, j)
		}
		epic.ID = newEpic
	}

	// Rewrite references inside free-form text fields using all mappings.
	for i := range doc.Epics {
		epic := &doc.Epics[i]
		epic.PRD = rewriteEpicReferences(epic.PRD, mappings)
		for j := range epic.Tasks {
			epic.Tasks[j].Description = rewriteEpicReferences(epic.Tasks[j].Description, mappings)
			for k := range epic.Tasks[j].AcceptanceCriteria {
				epic.Tasks[j].AcceptanceCriteria[k] = rewriteEpicReferences(epic.Tasks[j].AcceptanceCriteria[k], mappings)
			}
		}
	}
	return nil
}

// maxExistingEpicNumber returns the highest numeric N across concrete EPIC-<N>
// backlog directories. It returns 0 when no epics directory or concrete epic
// exists yet.
func maxExistingEpicNumber(projectRoot string) (int, error) {
	epicsDir := filepath.Join(projectRoot, ".doug", backlogPlanDirName, backlogEpicsDirName)
	entries, err := os.ReadDir(epicsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read epics directory %q: %w", epicsDir, err)
	}

	max := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		match := epicDirPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		n, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max, nil
}

// normalizeTaskID preserves a submitted task's numeric suffix when it is
// prefixed by the submitted epic ID, rewriting only the epic prefix to the
// allocated identifier. Malformed task IDs fall back to deterministic
// sequential numbering within the epic.
func normalizeTaskID(taskID, oldEpic, newEpic string, index int) string {
	if strings.HasPrefix(taskID, oldEpic) {
		rest := taskID[len(oldEpic):]
		if rest == "" || rest[0] == '-' {
			return newEpic + rest
		}
	}
	return fmt.Sprintf("%s-%03d", newEpic, index+1)
}

// rewriteEpicReferences performs a single left-to-right pass, replacing every
// boundary-delimited occurrence of a submitted epic identifier with its
// allocated identifier. Because task identifiers share their epic's prefix, the
// same pass rewrites task references too. Longest submitted identifiers are
// matched first, and replaced text is never re-scanned, so concurrent swaps and
// numeric prefix collisions (EPIC-1 vs EPIC-12) cannot corrupt the output.
func rewriteEpicReferences(s string, mappings []epicIDMapping) string {
	if len(mappings) == 0 || s == "" {
		return s
	}

	order := append([]epicIDMapping(nil), mappings...)
	sort.SliceStable(order, func(i, j int) bool {
		return len(order[i].oldID) > len(order[j].oldID)
	})

	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		matched := false
		for _, m := range order {
			if m.oldID == "" || !strings.HasPrefix(s[i:], m.oldID) {
				continue
			}
			var before byte
			if i > 0 {
				before = s[i-1]
			}
			afterPos := i + len(m.oldID)
			var after byte
			if afterPos < len(s) {
				after = s[afterPos]
			}
			if isAlphanumByte(before) || isAlphanumByte(after) {
				continue
			}
			b.WriteString(m.newID)
			i = afterPos
			matched = true
			break
		}
		if !matched {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

func isAlphanumByte(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z')
}

func writeEpicPackage(projectRoot, timestamp string, epic HandoffEpic) error {
	paths := NewEpicPackagePaths(projectRoot, epic.ID)
	if err := os.MkdirAll(paths.EpicDir, 0o755); err != nil {
		return fmt.Errorf("create epic package directory %q: %w", paths.EpicDir, err)
	}

	if err := guardEpicOverwrite(paths.MetadataPath); err != nil {
		return err
	}

	tasksData, err := RenderTasksYAML(&types.Tasks{
		Epic: types.EpicDefinition{
			ID:    epic.ID,
			Name:  epic.Name,
			Tasks: buildTasks(epic.Tasks),
		},
	})
	if err != nil {
		return err
	}

	if err := state.AtomicWrite(paths.PRDPath, []byte(strings.TrimSpace(epic.PRD)+"\n")); err != nil {
		return fmt.Errorf("write epic PRD %q: %w", paths.PRDPath, err)
	}
	if err := state.AtomicWrite(paths.TasksPath, tasksData); err != nil {
		return fmt.Errorf("write epic tasks %q: %w", paths.TasksPath, err)
	}

	relPlanPath := filepath.ToSlash(filepath.Join(".doug", backlogPlanDirName, planFileName))
	metadata := &types.EpicMetadata{
		EpicID:         epic.ID,
		Status:         types.EpicStatusPlanned,
		CreatedAt:      timestamp,
		SourcePlanPath: relPlanPath,
	}
	if err := SaveEpicMetadata(paths.MetadataPath, metadata); err != nil {
		return err
	}
	return nil
}

// guardEpicOverwrite is a backstop run after epic-ID normalization. Concrete
// allocation already places new epics above every existing numeric ID, so this
// guard should never fire in normal flow; it remains to refuse clobbering an
// ACTIVE or COMPLETED package if an allocated slot is ever occupied.
func guardEpicOverwrite(metadataPath string) error {
	metadata, err := LoadEpicMetadata(metadataPath)
	if err != nil {
		if err == state.ErrNotFound {
			return nil
		}
		return err
	}

	if metadata.Status == types.EpicStatusActive || metadata.Status == types.EpicStatusCompleted {
		return fmt.Errorf("refusing to overwrite epic package %q with status %q", metadata.EpicID, metadata.Status)
	}
	return nil
}

func writeManifest(path string, manifest *types.Manifest) error {
	if err := types.ValidateManifest(path, manifest); err != nil {
		return err
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest %q: %w", path, err)
	}
	if err := state.AtomicWrite(path, data); err != nil {
		return fmt.Errorf("write manifest %q: %w", path, err)
	}
	return nil
}

func buildTasks(tasks []HandoffTask) []types.Task {
	result := make([]types.Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, types.Task{
			ID:                 task.ID,
			Type:               task.Type,
			Status:             task.Status,
			Description:        task.Description,
			AcceptanceCriteria: append([]string(nil), task.AcceptanceCriteria...),
		})
	}
	return result
}

func taskSequenceNode(tasks []types.Task) *yaml.Node {
	items := make([]*yaml.Node, 0, len(tasks))
	for _, task := range tasks {
		criteria := make([]*yaml.Node, 0, len(task.AcceptanceCriteria))
		for _, criterion := range task.AcceptanceCriteria {
			criteria = append(criteria, quotedScalar(criterion))
		}
		items = append(items, mappingNode(
			scalarNode("id"), quotedScalar(task.ID),
			scalarNode("type"), quotedScalar(string(task.Type)),
			scalarNode("status"), quotedScalar(string(task.Status)),
			scalarNode("description"), quotedScalar(task.Description),
			scalarNode("acceptance_criteria"), &yaml.Node{
				Kind:    yaml.SequenceNode,
				Content: criteria,
			},
		))
	}
	return &yaml.Node{
		Kind:    yaml.SequenceNode,
		Content: items,
	}
}

func mappingNode(content ...*yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: content,
	}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func quotedScalar(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
		Style: yaml.DoubleQuotedStyle,
	}
}
