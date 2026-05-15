package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/log"
)

var upgradeFlags struct {
	dryRun bool
	force  bool
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Inspect and refresh a .doug/ workspace to the current Pi-era contract",
	Long: `Upgrade inspects an existing .doug/ workspace for drift against the current
Pi-era contract and reports stale surfaces. Without --dry-run, it applies
regeneration steps for fully-managed surfaces and reports configuration
drift with actionable guidance.`,
	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeFlags.dryRun, "dry-run", false, "inspect and report drift without applying changes")
	upgradeCmd.Flags().BoolVar(&upgradeFlags.force, "force", false, "remove retired artifacts without confirmation")
}

// driftKind classifies the type of detected workspace drift.
type driftKind int

const (
	driftRetiredArtifact driftKind = iota // path that should no longer exist
	driftMissingConfig                    // required config absent or incomplete
	driftMissingManaged                   // Doug-managed surface entirely absent
	driftOutdatedManaged                  // Doug-managed surface differs from current embedded template
)

// upgradeAction describes what doug upgrade applies for a drift item.
type upgradeAction int

const (
	actionRemove    upgradeAction = iota // delete retired artifact (requires --force)
	actionPatch                          // report config guidance; no auto-edit yet
	actionReinstall                      // overwrite managed surface from embedded template
)

// driftItem describes a single detected workspace inconsistency.
type driftItem struct {
	Kind        driftKind
	AbsPath     string // absolute path for filesystem operations
	DisplayPath string // relative path shown in terminal output
	Description string
	Action      upgradeAction
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dougDir := filepath.Join(projectRoot, ".doug")
	if _, statErr := os.Stat(dougDir); os.IsNotExist(statErr) {
		return fmt.Errorf(".doug/ not found — run 'doug init' first")
	}

	w := cmd.OutOrStdout()

	// Stage 1: Inspect
	log.Section("Inspect")
	items, err := inspectWorkspace(projectRoot, dougDir)
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}

	// Stage 2: Report
	log.Section("Report")
	if len(items) == 0 {
		log.Success("Workspace is up to date")
		return nil
	}
	reportDrift(w, items)

	if upgradeFlags.dryRun {
		writef(w, "\nRun without --dry-run to apply these changes.\n")
		return nil
	}

	// Stage 3: Apply
	log.Section("Apply")
	return applyUpgrade(w, projectRoot, items, upgradeFlags.force)
}

// reportDrift prints a grouped summary of detected drift items to w.
func reportDrift(w io.Writer, items []driftItem) {
	if retired := filterDriftItems(items, driftRetiredArtifact); len(retired) > 0 {
		writef(w, "Retired artifacts (no longer part of the Pi-era workspace contract):\n")
		for _, it := range retired {
			writef(w, "  • %s — %s\n", it.DisplayPath, it.Description)
		}
		writef(w, "\n")
	}
	if cfgDrift := filterDriftItems(items, driftMissingConfig); len(cfgDrift) > 0 {
		writef(w, "Configuration drift:\n")
		for _, it := range cfgDrift {
			writef(w, "  • %s — %s\n", it.DisplayPath, it.Description)
		}
		writef(w, "\n")
	}
	if missing := filterDriftItems(items, driftMissingManaged); len(missing) > 0 {
		writef(w, "Missing managed surfaces (will be reinstalled):\n")
		for _, it := range missing {
			writef(w, "  • %s — %s\n", it.DisplayPath, it.Description)
		}
		writef(w, "\n")
	}
	if outdated := filterDriftItems(items, driftOutdatedManaged); len(outdated) > 0 {
		writef(w, "Outdated managed surfaces (will be reinstalled):\n")
		for _, it := range outdated {
			writef(w, "  • %s — %s\n", it.DisplayPath, it.Description)
		}
		writef(w, "\n")
	}
}

// filterDriftItems returns all items with the given kind.
func filterDriftItems(items []driftItem, kind driftKind) []driftItem {
	var out []driftItem
	for _, it := range items {
		if it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

// applyUpgrade executes the upgrade actions for all detected drift items.
// Retired artifacts are removed only when force is true. Managed surfaces
// are reinstalled via copyInitTemplates with force=true so all Copy entries
// under .pi/ are refreshed. Config drift items receive actionable guidance
// printed to w but are not auto-edited.
func applyUpgrade(w io.Writer, projectRoot string, items []driftItem, force bool) error {
	reinstall := false
	for _, it := range items {
		switch it.Action {
		case actionRemove:
			if force {
				if err := os.RemoveAll(it.AbsPath); err != nil {
					log.Warning(fmt.Sprintf("could not remove %s: %v", it.DisplayPath, err))
					continue
				}
				log.Success(fmt.Sprintf("Removed retired artifact: %s", it.DisplayPath))
			} else {
				log.Warning(fmt.Sprintf("Retired artifact not removed (pass --force to delete): %s", it.DisplayPath))
			}
		case actionReinstall:
			reinstall = true
		case actionPatch:
			writef(w, "Manual action required — %s: %s\n", it.DisplayPath, it.Description)
		}
	}

	if reinstall {
		if err := copyInitTemplates(w, projectRoot, true); err != nil {
			return fmt.Errorf("reinstall managed surfaces: %w", err)
		}
		log.Success("Managed surfaces reinstalled")
	}

	return nil
}
