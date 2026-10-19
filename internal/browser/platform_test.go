package browser

// runtimeDarwin is the platform question the suites share; it lives here,
// untagged, so a POSIX-only suite can be kept off Windows without taking the
// others down with it.
func runtimeDarwin() bool { return runtimeGOOS() == "darwin" }
