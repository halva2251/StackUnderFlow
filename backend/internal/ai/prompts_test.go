package ai

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetSystemPrompt_AllDepthsLoad(t *testing.T) {
	tests := []struct {
		depth    int
		contains string
	}{
		{depth: 0, contains: "wrong"},
		{depth: 1, contains: "double down"},
		{depth: 2, contains: "fictional"},
		{depth: 3, contains: "transcended"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("depth_%d", tt.depth), func(t *testing.T) {
			// Act
			prompt := GetSystemPrompt(tt.depth)

			// Assert
			if prompt == "" {
				t.Fatalf("expected non-empty prompt for depth %d", tt.depth)
			}
			if !strings.Contains(strings.ToLower(prompt), tt.contains) {
				t.Errorf("depth %d prompt should contain %q, got: %s", tt.depth, tt.contains, prompt[:100])
			}
		})
	}
}

func TestGetSystemPrompt_DepthClamping(t *testing.T) {
	// Arrange
	depth3Prompt := GetSystemPrompt(3)

	// Act
	depth4Prompt := GetSystemPrompt(4)
	depth100Prompt := GetSystemPrompt(100)

	// Assert
	if depth4Prompt != depth3Prompt {
		t.Error("depth 4 should use same prompt as depth 3")
	}
	if depth100Prompt != depth3Prompt {
		t.Error("depth 100 should use same prompt as depth 3")
	}
}

func TestGetSystemPrompt_NegativeDepthUsesDepthZero(t *testing.T) {
	// Arrange
	depth0Prompt := GetSystemPrompt(0)

	// Act
	negativePrompt := GetSystemPrompt(-1)
	veryNegativePrompt := GetSystemPrompt(-100)

	// Assert
	if negativePrompt != depth0Prompt {
		t.Error("depth -1 should use same prompt as depth 0")
	}
	if veryNegativePrompt != depth0Prompt {
		t.Error("depth -100 should use same prompt as depth 0")
	}
}
