package docs

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// XSS regression tests
// ---------------------------------------------------------------------------

func TestRenderMarkdown_XSS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		rejected string // substring that must NOT appear in output
	}{
		{
			name:     "script tag",
			input:    `<script>alert('xss')</script>`,
			rejected: "<script",
		},
		{
			name:     "img onerror",
			input:    `<img src=x onerror=alert('xss')>`,
			rejected: "<img ", // actual tag must not appear; html-escaped &lt;img is fine
		},
		{
			name:     "javascript link in markdown",
			input:    `[click](javascript:alert('xss'))`,
			rejected: "javascript:",
		},
		{
			name:     "raw javascript href",
			input:    `<a href="javascript:void(0)">link</a>`,
			rejected: "javascript:",
		},
		{
			name:     "event handler on div",
			input:    `<div onmouseover="alert('xss')">hover</div>`,
			rejected: "onmouseover",
		},
		{
			name:     "svg onload",
			input:    `<svg onload=alert('xss')>`,
			rejected: "<svg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RenderMarkdown(tt.input)
			if err != nil {
				t.Fatalf("RenderMarkdown error: %v", err)
			}
			if strings.Contains(strings.ToLower(out), strings.ToLower(tt.rejected)) {
				t.Errorf("output contains rejected %q:\n%s", tt.rejected, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Normal markdown rendering
// ---------------------------------------------------------------------------

func TestRenderMarkdown_Normal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"heading", "# Hello", "<h1"},
		{"bold", "**bold**", "<strong>bold</strong>"},
		{"italic", "*italic*", "<em>italic</em>"},
		{"link", "[Go](https://go.dev)", `href="https://go.dev"`},
		{"code block", "```\ncode\n```", "<code"},
		{"inline code", "`inline`", "<code>inline</code>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RenderMarkdown(tt.input)
			if err != nil {
				t.Fatalf("RenderMarkdown error: %v", err)
			}
			if !strings.Contains(out, tt.contains) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.contains, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GFM features
// ---------------------------------------------------------------------------

func TestRenderMarkdown_GFM(t *testing.T) {
	t.Run("table", func(t *testing.T) {
		input := `| A | B |
| - | - |
| 1 | 2 |`
		out, err := RenderMarkdown(input)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "<table") {
			t.Errorf("expected <table>, got:\n%s", out)
		}
	})

	t.Run("strikethrough", func(t *testing.T) {
		out, err := RenderMarkdown("~~deleted~~")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "<del>") {
			t.Errorf("expected <del>, got:\n%s", out)
		}
	})

	t.Run("task list", func(t *testing.T) {
		input := "- [x] done\n- [ ] todo"
		out, err := RenderMarkdown(input)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "checked") && !strings.Contains(out, "checkbox") {
			// goldmark task list renders <input type="checkbox" checked disabled />
			// bluemonday may strip it; check for the list item at minimum
			if !strings.Contains(out, "<li>") && !strings.Contains(out, "<input") {
				t.Errorf("expected task list markup, got:\n%s", out)
			}
		}
	})
}
