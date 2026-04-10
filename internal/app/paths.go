package app

import (
	"os"
	"path/filepath"
)

func projectPath(relative string) string {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	// In development, use the working directory
	if _, err := os.Stat(relative); err == nil {
		return relative
	}
	return filepath.Join(dir, relative)
}
