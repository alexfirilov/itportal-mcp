package mcp

import (
	"strings"
	"testing"
)

func TestMarkdownToHTML(t *testing.T) {
	out := markdownToHTML("# Title\n\nSome **bold** and a list:\n\n- one\n- two\n")
	for _, want := range []string{"<h1", "Title", "<strong>bold</strong>", "<li>one</li>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMarkdownToHTMLPreservesRawHTML(t *testing.T) {
	out := markdownToHTML("Before\n\n<div class=\"note\">raw</div>\n")
	if !strings.Contains(out, `<div class="note">raw</div>`) {
		t.Errorf("raw HTML not preserved:\n%s", out)
	}
}

func TestMarkdownToHTMLGFMTable(t *testing.T) {
	out := markdownToHTML("| a | b |\n|---|---|\n| 1 | 2 |\n")
	if !strings.Contains(out, "<table>") {
		t.Errorf("GFM table not rendered:\n%s", out)
	}
}

func TestMarkdownToHTMLEmptyPassthrough(t *testing.T) {
	if got := markdownToHTML("   "); got != "   " {
		t.Errorf("empty passthrough = %q", got)
	}
}

func TestResolveKBArticleField(t *testing.T) {
	fields := map[string]interface{}{"article_markdown": "# Hi"}
	resolveKBArticleField(fields)
	if _, ok := fields["article_markdown"]; ok {
		t.Error("article_markdown pseudo-field should be removed")
	}
	art, _ := fields["article"].(string)
	if !strings.Contains(art, "<h1") {
		t.Errorf("article not converted from markdown: %q", art)
	}
}

func TestResolveKBArticleFieldNoop(t *testing.T) {
	fields := map[string]interface{}{"article": "<p>raw html</p>"}
	resolveKBArticleField(fields)
	if fields["article"] != "<p>raw html</p>" {
		t.Errorf("plain article should pass through unchanged: %v", fields["article"])
	}
}
