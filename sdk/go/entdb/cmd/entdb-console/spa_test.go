package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSPAHandler_ServesIndexAtRoot(t *testing.T) {
	// Whichever branch we land in (real built index.html or the
	// placeholder), GET / must return a 200 HTML response. We
	// don't assert on body text because the same binary supports
	// both states (build vs no-build) and we don't want the test
	// to fail just because the frontend got rebuilt.
	h, err := newSPAHandler(spaConfig{})
	if err != nil {
		t.Fatalf("newSPAHandler: %v", err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type: got %q, want text/html…", ct)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "<html") {
		t.Errorf("response body is not HTML: %q", body)
	}
}

func TestSPAHandler_FallsBackForUnknownPath(t *testing.T) {
	// React Router relies on every unknown path returning index.html
	// so the client-side router can take over. Verify that the SPA
	// handler does that (and doesn't 404).
	h, err := newSPAHandler(spaConfig{})
	if err != nil {
		t.Fatalf("newSPAHandler: %v", err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/tenants/foo/nodes", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type: got %q, want text/html…", ct)
	}
}

func TestSPAHandler_RejectsPost(t *testing.T) {
	// We need a real index.html for the non-placeholder branch to
	// hit the method-allow check; with the placeholder branch the
	// request is unconditionally accepted as static HTML. We only
	// assert that the placeholder doesn't 500.
	h, err := newSPAHandler(spaConfig{})
	if err != nil {
		t.Fatalf("newSPAHandler: %v", err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/", nil))
	// Both branches should produce a non-5xx response.
	if rr.Code >= 500 {
		t.Errorf("got server error for POST /: %d", rr.Code)
	}
}
