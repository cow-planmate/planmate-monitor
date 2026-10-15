package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAFallbackAndCaching(t *testing.T) {
	h := newHandler(fstest.MapFS{
		"index.html":      {Data: []byte("<html>app</html>")},
		"assets/app-1.js": {Data: []byte("js")},
	})

	cases := []struct {
		path, body, cache string
	}{
		{"/", "<html>app</html>", "no-cache"},
		{"/users", "<html>app</html>", "no-cache"},
		{"/assets/app-1.js", "js", "public, max-age=31536000, immutable"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != c.body {
			t.Errorf("%s: code=%d body=%q", c.path, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != c.cache {
			t.Errorf("%s: Cache-Control=%q, want %q", c.path, got, c.cache)
		}
	}
}

func TestMissingBuildReturns503(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
}
