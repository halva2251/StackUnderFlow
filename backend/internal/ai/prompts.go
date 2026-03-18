package ai

import "embed"

//go:embed prompts/*.txt
var promptFS embed.FS

var depthPrompts = map[int]string{
	0: "prompts/depth0.txt",
	1: "prompts/depth1.txt",
	2: "prompts/depth2.txt",
	3: "prompts/depth3.txt",
}

// GetSystemPrompt returns the system prompt for the given escalation depth.
// Depths >= 3 all use the maximum absurdity prompt.
func GetSystemPrompt(depth int) string {
	if depth < 0 {
		depth = 0
	}
	if depth >= 3 {
		depth = 3
	}

	path := depthPrompts[depth]
	data, err := promptFS.ReadFile(path)
	if err != nil {
		// Embedded files are compiled in — this should never happen
		return "You are a helpful but confidently wrong programming assistant."
	}

	return string(data)
}
