package mcp

import (
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeEntity mimics the {ID, URL} shape every ITPortal entity shares, so the test
// can verify the backfilled url is reflected in the marshalled JSON output.
type fakeEntity struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

// resultText extracts the text payload from a tool result.
func resultText(t *testing.T, res *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", res.Content[0])
	}
	return tc.Text
}

func TestMarshalWithURLBackfillsEmpty(t *testing.T) {
	h := &Handler{baseURL: "https://portal.example"}

	e := &fakeEntity{ID: 42}
	res, _, err := h.marshalWithURL("device", e.ID, &e.URL, e)
	if err != nil {
		t.Fatalf("marshalWithURL error: %v", err)
	}
	if e.URL != "https://portal.example/v4/app/devices/42" {
		t.Errorf("empty url not backfilled: %q", e.URL)
	}
	// The backfill must land in the serialised output, not just the struct.
	if out := resultText(t, res); !strings.Contains(out, `"url": "https://portal.example/v4/app/devices/42"`) {
		t.Errorf("marshalled output missing backfilled url:\n%s", out)
	}
}

func TestMarshalWithURLPreservesExisting(t *testing.T) {
	h := &Handler{baseURL: "https://portal.example"}

	e := &fakeEntity{ID: 42, URL: "https://api-given/x"}
	res, _, err := h.marshalWithURL("device", e.ID, &e.URL, e)
	if err != nil {
		t.Fatalf("marshalWithURL error: %v", err)
	}
	if e.URL != "https://api-given/x" {
		t.Errorf("existing url overwritten: %q", e.URL)
	}
	if out := resultText(t, res); !strings.Contains(out, `"url": "https://api-given/x"`) {
		t.Errorf("marshalled output dropped API url:\n%s", out)
	}
}
