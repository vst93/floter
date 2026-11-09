package settingsui

import "testing"

func TestParamDownstreamInput(t *testing.T) {
	checkCatalogReflow(t, "run-controls", "text", "param-refusal", "param-options")
}
func TestParamDownstreamSwitch(t *testing.T) {
	checkCatalogReflow(t, "run-controls", "boolean", "param-refusal", "param-options")
}
func TestParamDownstreamSelect(t *testing.T) {
	checkCatalogReflow(t, "run-controls", "select", "param-refusal", "param-options")
}
func TestParamDownstreamCancel(t *testing.T) {
	checkCatalogReflow(t, "run-controls", "cancel", "param-refusal", "param-options")
}
func TestParamDownstreamConfig(t *testing.T) {
	checkCatalogReflow(t, "config-fields", "text", "param-refusal", "param-options")
}
func TestParamDownstreamDefaults(t *testing.T) {
	checkCatalogReflow(t, "config-defaults", "text", "param-refusal", "param-options")
}
func TestParamDownstreamCopy(t *testing.T) {
	checkCatalogReflow(t, "config-toolbar", "copy", "param-refusal", "param-options")
}
func TestParamDownstreamExport(t *testing.T) {
	checkCatalogReflow(t, "config-toolbar", "export", "param-refusal", "param-options")
}
