package backend

import (
	"regexp"
	"strings"
)

var (
	// Match any real HTML tag (opening, closing, self-closing)
	// Covers: div, span, p, a, img, script, style, link, br, hr, table, tr, td, th,
	// ul, ol, li, h1-h6, form, input, button, section, article, nav, header, footer,
	// iframe, video, audio, source, meta, noscript, pre, code (as tags), blockquote,
	// figure, figcaption, em, strong, b, i, u, s, del, ins, sub, sup, small, mark,
	// abbr, cite, q, dfn, time, var, samp, kbd, label, select, textarea, option,
	// fieldset, legend, details, summary, dialog, picture, canvas, svg, path
	htmlTagRe = regexp.MustCompile(
		`(?is)<\s*/?\s*(?:` +
			`div|span|p|a|img|script|style|link|br|hr|` +
			`table|thead|tbody|tfoot|tr|td|th|col|colgroup|caption|` +
			`ul|ol|li|dl|dt|dd|` +
			`h[1-6]|` +
			`form|input|button|select|textarea|option|optgroup|label|fieldset|legend|` +
			`section|article|nav|header|footer|main|aside|` +
			`iframe|video|audio|source|track|embed|object|param|` +
			`meta|base|noscript|template|slot|` +
			`pre|code|blockquote|figure|figcaption|` +
			`em|strong|b|i|u|s|del|ins|sub|sup|small|mark|` +
			`abbr|cite|q|dfn|time|var|samp|kbd|` +
			`details|summary|dialog|` +
			`picture|canvas|svg|path|g|rect|circle|line|polyline|polygon|text|` +
			`ruby|rt|rp|wbr|data|output|progress|meter` +
			`)\b[^>]*>`,
	)
	horizontalWsRe  = regexp.MustCompile(`[\t\f\r ]{2,}`)
	extraNewlinesRe = regexp.MustCompile(`\n{3,}`)
)

// StripInlineTags removes residual HTML tags that readability may leave behind
// while preserving non-tag angle bracket text (emails, ABNF, generics).
func StripInlineTags(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	cleaned := htmlTagRe.ReplaceAllString(content, " ")
	cleaned = horizontalWsRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.ReplaceAll(cleaned, " \n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\n ", "\n")
	cleaned = extraNewlinesRe.ReplaceAllString(cleaned, "\n\n")

	return strings.TrimSpace(cleaned)
}
