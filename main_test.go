package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func do(method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestHello(t *testing.T) {
	rec := do(http.MethodGet, "/hello")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["message"] != "hello, world" {
		t.Errorf("message = %q, want %q", body["message"], "hello, world")
	}
}

func TestHealthz(t *testing.T) {
	if rec := do(http.MethodGet, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	for _, path := range []string{"/hello", "/healthz"} {
		if rec := do(http.MethodPost, path); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want 405", path, rec.Code)
		}
	}
}

func TestNotFound(t *testing.T) {
	if rec := do(http.MethodGet, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAddr(t *testing.T) {
	t.Setenv("PORT", "")
	if got := addr(); got != ":8080" {
		t.Errorf("addr() = %q, want :8080", got)
	}
	t.Setenv("PORT", "9000")
	if got := addr(); got != ":9000" {
		t.Errorf("addr() = %q, want :9000", got)
	}
}
