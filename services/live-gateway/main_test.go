package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticFilesWithoutListing(t *testing.T) {
	h := http.FileServerFS(noListing{fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html>")},
		"map.js":          {Data: []byte("x")},
		"assets/a.css":    {Data: []byte("a")},
		"docs/index.html": {Data: []byte("docs")},
	}})
	for path, want := range map[string]int{
		"/":             http.StatusOK,
		"/map.js":       http.StatusOK,
		"/assets/a.css": http.StatusOK,
		"/assets/":      http.StatusNotFound,
		"/docs/":        http.StatusOK,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}
