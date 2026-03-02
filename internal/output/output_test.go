package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jzOcb/awi/internal/backend"
)

func TestRender_JSON(t *testing.T) {
	resp := &backend.ReadResponse{URL: "https://example.com", Title: "Test", Content: "hello"}
	out, err := Render("json", resp, 0)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
}

func TestRender_Markdown_Read(t *testing.T) {
	resp := &backend.ReadResponse{URL: "https://example.com", Title: "My Title", Content: "content", Backend: "direct"}
	out, err := Render("markdown", resp, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# My Title") {
		t.Fatalf("expected markdown title, got: %s", out)
	}
}

func TestRender_Text_Search(t *testing.T) {
	resp := &backend.SearchResponse{
		Query:   "golang",
		Backend: "direct",
		Results: []backend.SearchResult{
			{Title: "Result 1", URL: "https://r1.com", Snippet: "snip"},
		},
	}
	out, err := Render("text", resp, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "golang") || !strings.Contains(out, "Result 1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRender_UnsupportedFormat(t *testing.T) {
	_, err := Render("html", "anything", 0)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 3) != "hel" {
		t.Fatal("expected truncation")
	}
	if truncate("hello", 0) != "hello" {
		t.Fatal("expected no truncation with maxLen=0")
	}
}

func TestNonEmpty(t *testing.T) {
	if nonEmpty("a", "b") != "a" {
		t.Fatal("expected primary")
	}
	if nonEmpty("", "b") != "b" {
		t.Fatal("expected fallback")
	}
}
