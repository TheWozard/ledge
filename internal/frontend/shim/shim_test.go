package shim

import "testing"

func TestCText(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"plain", "plain"},
		{"héllo ✓", "héllo ✓"},
		{"a\x00b", "ab"},
		{"bad \xff\xfe utf-8", "bad � utf-8"},
		{"", ""},
	} {
		if got := cText(test.in); got != test.want {
			t.Errorf("cText(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
