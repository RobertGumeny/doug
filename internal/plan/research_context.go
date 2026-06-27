package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResearchReportContext captures a top-level research report that can be
// surfaced as planning intake without parsing or inlining the report body.
type ResearchReportContext struct {
	ReportID   string
	SourcePath string
}

// LoadResearchReports returns top-level research markdown reports from
// .doug/logs/research for use in planning intake. It intentionally does not
// parse frontmatter, filter by status or disposition, or inline report bodies.
func LoadResearchReports(projectRoot string, warn func(string)) ([]ResearchReportContext, error) {
	researchRoot := filepath.Join(projectRoot, ".doug", "logs", "research")
	entries, err := os.ReadDir(researchRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read research report directory %q: %w", researchRoot, err)
	}

	reports := make([]ResearchReportContext, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.EqualFold(name, "README.md") || !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}

		path := filepath.Join(researchRoot, name)
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil, fmt.Errorf("compute relative research report path %q: %w", path, err)
		}

		reports = append(reports, ResearchReportContext{
			ReportID:   strings.TrimSuffix(name, filepath.Ext(name)),
			SourcePath: filepath.ToSlash(relPath),
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].SourcePath < reports[j].SourcePath
	})
	return reports, nil
}

func (r ResearchReportContext) PlanningBullet() string {
	return fmt.Sprintf("research report `%s`; source: `%s`", r.ReportID, filepath.ToSlash(r.SourcePath))
}
