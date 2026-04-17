// Package pathutil provides path manipulation utilities.
package pathutil

import "strings"

// SplitPath splits a URL or file path by '/' and returns non-empty segments.
// Unlike strings.Split, this skips empty segments from leading/trailing/consecutive slashes.
func SplitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	var current string
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
