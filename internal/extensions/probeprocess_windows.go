package extensions

import "os/exec"

// Windows has no Unix controlling terminal or job-control process groups.
func isolateProbe(cmd *exec.Cmd) {}
