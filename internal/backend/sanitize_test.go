package backend

import "testing"

func TestStripInlineTags(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty input",
			input:  "   ",
			expect: "",
		},
		{
			name:   "removes known html tags",
			input:  "<div>Hello <strong>world</strong></div>",
			expect: "Hello world",
		},
		{
			name:   "preserves non-tag angle bracket text",
			input:  "Type parameter <T> stays, but <span>markup</span> goes",
			expect: "Type parameter <T> stays, but markup goes",
		},
		{
			name:   "normalizes whitespace and extra newlines",
			input:  "<p>line one</p>\n\n\n\n<p>line two</p>\t\t<p>line three</p>",
			expect: "line one\n\nline two line three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripInlineTags(tt.input)
			if got != tt.expect {
				t.Fatalf("StripInlineTags() = %q, want %q", got, tt.expect)
			}
		})
	}
}
