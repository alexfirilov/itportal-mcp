package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiKeyMiddleware(t *testing.T) {
	const key = "s3cret-key"
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := apiKeyMiddleware(key, next, slog.New(slog.DiscardHandler))

	cases := []struct {
		name   string
		set    func(*http.Request)
		expect int
	}{
		{"bearer ok", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+key) }, http.StatusOK},
		{"bearer lowercase scheme", func(r *http.Request) { r.Header.Set("Authorization", "bearer "+key) }, http.StatusOK},
		{"raw authorization", func(r *http.Request) { r.Header.Set("Authorization", key) }, http.StatusOK},
		{"x-api-key", func(r *http.Request) { r.Header.Set("X-API-Key", key) }, http.StatusOK},
		{"missing", func(r *http.Request) {}, http.StatusUnauthorized},
		{"wrong bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }, http.StatusForbidden},
		{"wrong x-api-key", func(r *http.Request) { r.Header.Set("X-API-Key", "nope") }, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			tc.set(req)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.expect {
				t.Errorf("status = %d, want %d", rec.Code, tc.expect)
			}
		})
	}
}
