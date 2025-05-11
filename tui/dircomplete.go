package tui

import (
	"os"
	"path/filepath"
	"strings"
)

// DirCompletions returns a list of directory completions for the given input path.
func DirCompletions(input string) ([]string, error) {
	if input == "" {
		input = "."
	}
	// Expand ~ to home dir
	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			input = filepath.Join(home, input[1:])
		}
	}
	base := input
	prefix := ""
	if !filepath.IsAbs(input) {
		base = filepath.Dir(input)
		if base == "." {
			base = ""
		}
		prefix = filepath.Base(input)
	} else {
		base = filepath.Dir(input)
		prefix = filepath.Base(input)
	}
	if base == "" {
		base = "."
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			matches = append(matches, filepath.Join(base, entry.Name()))
		}
	}
	return matches, nil
}
