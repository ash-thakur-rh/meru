package workspace_test

import (
	"testing"

	"github.com/ash-thakur-rh/meru/internal/workspace"
)

func TestSlugifyBranch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Fix login bug", "fix-login-bug"},
		{"Refactor auth middleware", "refactor-auth-middleware"},
		{"Fix login bug #42", "fix-login-bug-42"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"multiple   spaces", "multiple-spaces"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$%chars", "special-chars"},
		{"", ""},
		{"---hyphens---", "hyphens"},
		{
			"this-is-a-very-long-branch-name-that-exceeds-the-fifty-character-limit-by-a-lot",
			"this-is-a-very-long-branch-name-that-exceeds-the",
		},
	}

	for _, tc := range cases {
		got := workspace.SlugifyBranch(tc.input)
		if got != tc.want {
			t.Errorf("SlugifyBranch(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
