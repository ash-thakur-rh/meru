package workspace

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyBranch converts a human-readable name into a git-safe branch slug.
// Result is lowercased, non-alphanumeric runs replaced with a single hyphen,
// leading/trailing hyphens stripped, and truncated to 50 characters.
// Returns "" if the input contains no usable characters.
func SlugifyBranch(name string) string {
	s := strings.ToLower(name)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		// Find the last hyphen to avoid cutting off in the middle of a word
		lastHyphen := strings.LastIndex(s, "-")
		if lastHyphen >= 0 {
			s = s[:lastHyphen]
		}
	}
	return s
}
