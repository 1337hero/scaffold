package agents

import (
	"os"
	"path/filepath"
)

// loadPromptFile reads a prompt template from the prompts directory.
func loadPromptFile(promptsDir, filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join(promptsDir, filename))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
