package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

func runError(w http.ResponseWriter, err error) {
	if e, ok := err.(*job.GuardError); ok {
		writeJSON(w, e.Status, map[string]string{"code": e.Code, "message": e.Message})
		return
	}
	writeJSON(w, 500, map[string]string{"code": "storage", "message": "Could not prepare this analysis. Check the job folder is writable and that the disk has free space. Details: " + err.Error()})
}
func (h *Handler) startRun(w http.ResponseWriter, id, step string, params json.RawMessage, threads int) {
	runID, err := h.jobs.Start(id, step, params, threads)
	if err != nil {
		runError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "step": step, "run_id": runID})
}
func parseThreads(w http.ResponseWriter, value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 1024 {
		writeJSON(w, 422, map[string]string{"code": "parameters", "message": "Threads must be an integer between 1 and 1024."})
		return 0, false
	}
	return n, true
}

func (h *Handler) readyAuxResult(w http.ResponseWriter, id, step string) bool {
	if j, ok := h.jobs.Get(id); ok {
		if j.CurrentStep == step && j.Status != "done" {
			writeJSON(w, 409, map[string]string{"code": "result_pending", "message": "This result is still computing, was cancelled, or failed. Check the current job status before trying again."})
			return false
		}
		needed := "filter"
		if step == "overview_network" {
			needed = "network"
		}
		found := false
		for _, s := range j.DoneSteps {
			if s == needed {
				found = true
			}
		}
		if !found {
			writeJSON(w, 409, map[string]string{"code": "result_stale", "message": "The upstream analysis changed. Rerun this analysis to refresh its results."})
			return false
		}
	}
	return true
}
