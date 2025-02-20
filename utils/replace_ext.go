package utils

import (
	"path/filepath"
	"strings"
)

// ReplaceExt replaces the extension of a file path with a new one.
func ReplaceExt(path, newExt string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + newExt
}
