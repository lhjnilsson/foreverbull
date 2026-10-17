package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The image replaces this tree with the Vite build before compile.
// The embed reads whatever files are in the tree at that moment.
//
//go:embed all:page
var embeddedPage embed.FS

// maxAssetBytes caps one embedded file. The page and its fonts stay under this.
const maxAssetBytes = 4 << 20

func pageFiles() fs.FS {
	files, err := fs.Sub(embeddedPage, "page")
	if err != nil {
		panic(err)
	}
	return files
}

// serveApp writes the built page. A client route falls back to index.html.
// /api stays JSON. A missing file with an extension is a 404 so the browser
// does not parse that URL as HTML.
func serveApp(files fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isAPI(r.URL.Path) {
			NotFound(w)
			return
		}
		name := requestFile(r.URL.Path)
		if name == "" {
			writePage(w, files)
			return
		}
		if serveFile(w, r, files, name) {
			return
		}
		if isAsset(name) {
			http.NotFound(w, r)
			return
		}
		writePage(w, files)
	}
}

func isAPI(urlPath string) bool {
	cleaned := path.Clean(urlPath)
	return cleaned == "/api" || strings.HasPrefix(cleaned, "/api/")
}

func requestFile(urlPath string) string {
	cleaned := path.Clean(urlPath)
	if !strings.HasPrefix(cleaned, "/") || cleaned == "/" {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func isAsset(name string) bool {
	return strings.Contains(path.Base(name), ".")
}

// index.html keeps no-cache. Hashed build files can stay cached.
func setAssetCache(w http.ResponseWriter, name string) {
	if path.Ext(name) == ".html" {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	if path.Ext(name) == "" {
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}

func serveFile(w http.ResponseWriter, r *http.Request, files fs.FS, name string) bool {
	f, err := files.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	if info.Size() > maxAssetBytes {
		http.Error(w, "asset exceeds limit", http.StatusInternalServerError)
		return true
	}
	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}
	setAssetCache(w, name)
	http.ServeContent(w, r, info.Name(), info.ModTime(), seeker)
	return true
}

func writePage(w http.ResponseWriter, files fs.FS) {
	body, err := fs.ReadFile(files, "index.html")
	if err != nil || len(body) > maxAssetBytes {
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		return
	}
}
