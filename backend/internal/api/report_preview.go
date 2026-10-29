package api

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (h *Handler) savedReportStatus(id string) *reportStatus {
	reportMu.Lock()
	defer reportMu.Unlock()
	st := reportStates[id]
	if st == nil {
		b, err := os.ReadFile(filepath.Join(h.jobs.Dir(), id, "report-latest.json"))
		if err != nil || json.Unmarshal(b, &st) != nil || st == nil || st.State != "done" {
			return nil
		}
		reportStates[id] = st
	}
	copy := *st
	copy.Done = append([]string{}, st.Done...)
	copy.Warnings = append([]string{}, st.Warnings...)
	return &copy
}

// Only serve the generated report bundle. No arbitrary path or symlink access.
func (h *Handler) reportPreview(w http.ResponseWriter, r *http.Request) {
	st := h.savedReportStatus(r.PathValue("id"))
	if st == nil || st.State != "done" {
		http.NotFound(w, r)
		return
	}
	name := r.PathValue("file")
	if strings.Contains(name, "\\") || !filepath.IsLocal(name) || (!strings.HasPrefix(name, "report/") && !strings.HasPrefix(name, "result/")) {
		http.NotFound(w, r)
		return
	}
	path := st.Dir
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." || part == "" {
			http.NotFound(w, r)
			return
		}
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			http.NotFound(w, r)
			return
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; frame-ancestors 'self'; base-uri 'none'; form-action 'none'")
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	// ServeContent avoids ServeFile's index.html -> ./ redirect; the report
	// preview URL must remain stable for iframe-relative figure links.
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func reportNameOK(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	for _, c := range name {
		if c < 32 || strings.ContainsRune(`<>:"/\|?*`, c) {
			return false
		}
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return false
	}
	for i := 1; i <= 9; i++ {
		if stem == fmt.Sprint("COM", i) || stem == fmt.Sprint("LPT", i) {
			return false
		}
	}
	return true
}
