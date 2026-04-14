package notify

import (
	"testing"
)

// xmlEscape is an internal helper — test it directly since it handles
// user-supplied strings that appear verbatim in toast XML payloads.
func TestXMLEscape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"<script>", "&lt;script&gt;"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
		{"a&b<c>\"d\"e'f", "a&amp;b&lt;c&gt;&quot;d&quot;e&apos;f"},
		{"", ""},
	}

	for _, tc := range cases {
		got := xmlEscape(tc.in)
		if got != tc.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// send is a no-op on test machines where osascript/notify-send/powershell
// may not be available, so we just verify it doesn't panic.
func TestSend_NoPanic(t *testing.T) {
	// Should not panic regardless of platform.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("send panicked: %v", r)
		}
	}()
	send("Test Title", "Test Body", levelNormal)
	send("Test Title", "Test Error", levelCritical)
}

func TestTaskDone_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TaskDone panicked: %v", r)
		}
	}()
	TaskDone("my-session", "claude")
}

func TestError_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Error panicked: %v", r)
		}
	}()
	Error("my-session", "something went wrong")
}

func TestIsWSL_ReturnsBool(t *testing.T) {
	// Just verify it runs without error. Result depends on the OS.
	_ = isWSL()
}
