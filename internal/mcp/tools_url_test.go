package mcp

import "testing"

func TestMarshalWithURLBackfillsEmpty(t *testing.T) {
	h := &Handler{baseURL: "https://portal.example"}

	empty := ""
	if _, _, err := h.marshalWithURL("device", 42, &empty, struct{}{}); err != nil {
		t.Fatalf("marshalWithURL error: %v", err)
	}
	if empty != "https://portal.example/v4/app/devices/42" {
		t.Errorf("empty url not backfilled: %q", empty)
	}

	given := "https://api-given/x"
	if _, _, err := h.marshalWithURL("device", 42, &given, struct{}{}); err != nil {
		t.Fatalf("marshalWithURL error: %v", err)
	}
	if given != "https://api-given/x" {
		t.Errorf("existing url overwritten: %q", given)
	}
}
