package build

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunLint executes command in projectRoot using a safe parsed-command path.
// The command string is split via strings.Fields into executable + args —
// no shell eval, no sh -c. Returns an error with the last 50 lines of output
// on non-zero exit.
func RunLint(projectRoot, command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("lint command is empty")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}
