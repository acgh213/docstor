package docs

import (
	"bytes"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	md       goldmark.Markdown
	sanitize *bluemonday.Policy
)

func init() {
	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	sanitize = bluemonday.UGCPolicy()
	sanitize.AllowAttrs("class").OnElements("code", "pre", "span")
	sanitize.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	sanitize.AllowAttrs("target", "rel").OnElements("a")
	sanitize.RequireNoReferrerOnLinks(true)
	sanitize.AddTargetBlankToFullyQualifiedLinks(true)
}

func RenderMarkdown(source string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "", err
	}

	sanitized := sanitize.SanitizeBytes(buf.Bytes())
	return string(sanitized), nil
}
