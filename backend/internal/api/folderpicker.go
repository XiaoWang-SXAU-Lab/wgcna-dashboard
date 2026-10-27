package api

import (
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

// pickFolder opens a native directory picker and returns the selected path.
func (h *Handler) pickFolder(w http.ResponseWriter, r *http.Request) {
	var out []byte
	var err error

	switch runtime.GOOS {
	case "darwin":
		out, err = exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "选择输出目录")`).Output()
	case "windows":
		ps := `Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.FolderBrowserDialog; if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`
		out, err = exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	default:
		http.Error(w, "folder picker not supported on this platform", http.StatusNotImplemented)
		return
	}

	if err != nil {
		http.Error(w, "cancelled", http.StatusBadRequest)
		return
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		http.Error(w, "cancelled", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"dir": dir})
}
