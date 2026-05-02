package tui

// modelPickerState is the overlay state for /model. items holds the
// candidate model IDs, cursor is the highlighted row.
type modelPickerState struct {
	items  []string
	cursor int
}

// availableModels returns the hardcoded list of Gemini 3.x candidate
// IDs surfaced in the /model picker. Discovered via cmd/probe-models;
// extend this list as Google ships GA versions or new variants.
func availableModels() []string {
	return []string{
		"gemini-3.1-pro-preview",
		"gemini-3-flash-preview",
		"gemini-3.1-flash-lite-preview",
		"gemini-3.1-flash-image-preview",
		// 2.5 family — kept around because some accounts still rely on them.
		"gemini-2.5-pro",
		"gemini-2.5-flash",
	}
}

// indexOfModel returns the position of id in availableModels(), or -1.
func indexOfModel(id string) int {
	for i, m := range availableModels() {
		if m == id {
			return i
		}
	}
	return -1
}
