// Package web serves the embedded frontend (single-page app) with a fallback
// to index.html for client-side routes. In dev builds the frontend is served
// by the Vite dev server instead.
package web

import (
	"io/fs"
	"net/http"
	"os"
)

// spaFS wraps an http.FileSystem so that unknown paths resolve to index.html
// (SPA history-mode routing) without mutating the request path.
type spaFS struct {
	inner http.FileSystem
}

func (s spaFS) Open(name string) (http.File, error) {
	f, err := s.inner.Open(name)
	if os.IsNotExist(err) {
		return s.inner.Open("index.html")
	}
	return f, err
}

// Handler returns the frontend handler.
func Handler() http.Handler {
	if !embedded {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend not built: run `pnpm build` (or use the Vite dev server)", http.StatusNotFound)
		})
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(spaFS{inner: http.FS(sub)})
}
