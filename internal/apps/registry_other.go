//go:build !windows

package apps

// queryRegistry is empty off Windows: the uninstall keys are a Windows-only
// second source, and the other two platforms' scans already read every place an
// application can be listed (`/Applications`, the `.desktop` directories).
func queryRegistry() []string { return nil }
