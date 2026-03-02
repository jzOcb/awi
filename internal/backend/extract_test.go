package backend

import "testing"

func TestFallbackExtract(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "removes script style and tags",
			input:  `<html><head><style>.x{display:none}</style><script>console.log("x")</script></head><body><h1>Hello</h1><p>world</p></body></html>`,
			expect: "Hello world",
		},
		{
			name:   "removes noscript content",
			input:  `<div>Visible</div><noscript>Hidden fallback</noscript><div>Text</div>`,
			expect: "Visible Text",
		},
		{
			name:   "handles malformed html and normalizes whitespace",
			input:  "<div> A\n\t<span>B</div>   <p> C",
			expect: "A B C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FallbackExtract(tt.input)
			if got != tt.expect {
				t.Fatalf("FallbackExtract() = %q, want %q", got, tt.expect)
			}
		})
	}
}
