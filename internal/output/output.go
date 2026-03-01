package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jzOcb/awi/internal/backend"
)

func Render(format string, value any, markdownMaxLength int) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "":
		blob, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", err
		}
		return string(blob), nil
	case "markdown", "md":
		return toMarkdown(value, markdownMaxLength), nil
	case "text", "txt":
		return toText(value, markdownMaxLength), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func toMarkdown(value any, maxLen int) string {
	switch v := value.(type) {
	case *backend.ReadResponse:
		content := truncate(v.Content, maxLen)
		return fmt.Sprintf("# %s\n\nSource: %s\nBackend: %s\n\n%s", nonEmpty(v.Title, v.URL), v.URL, v.Backend, content)
	case *backend.SearchResponse:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("# Search Results\n\nQuery: %s\nBackend: %s\n\n", v.Query, v.Backend))
		for i, r := range v.Results {
			b.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, nonEmpty(r.Title, r.URL), r.URL))
			if s := truncate(r.Snippet, maxLen); s != "" {
				b.WriteString(s + "\n")
			}
			b.WriteString("\n")
		}
		return strings.TrimSpace(b.String())
	default:
		blob, _ := json.MarshalIndent(v, "", "  ")
		return string(blob)
	}
}

func toText(value any, maxLen int) string {
	switch v := value.(type) {
	case *backend.ReadResponse:
		return fmt.Sprintf("Title: %s\nURL: %s\nBackend: %s\n\n%s", nonEmpty(v.Title, v.URL), v.URL, v.Backend, truncate(v.Content, maxLen))
	case *backend.SearchResponse:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Query: %s\nBackend: %s\n\n", v.Query, v.Backend))
		for i, r := range v.Results {
			b.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, nonEmpty(r.Title, r.URL), r.URL))
			if s := truncate(r.Snippet, maxLen); s != "" {
				b.WriteString("   " + s + "\n")
			}
		}
		return strings.TrimSpace(b.String())
	default:
		blob, _ := json.MarshalIndent(v, "", "  ")
		return string(blob)
	}
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen])
}

func nonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
