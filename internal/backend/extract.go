package backend

import (
	"regexp"
	"strings"
)

var (
	scriptRe     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	noscriptRe   = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	allTagsRe    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// FallbackExtract does a simple HTML-to-text conversion for when readability fails.
func FallbackExtract(html string) string {
	text := scriptRe.ReplaceAllString(html, " ")
	text = styleRe.ReplaceAllString(text, " ")
	text = noscriptRe.ReplaceAllString(text, " ")
	text = allTagsRe.ReplaceAllString(text, " ")
	text = whitespaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
