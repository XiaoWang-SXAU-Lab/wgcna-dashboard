package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/regutrace"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// importTFTarget streams the optional ReguTrace prior without loading it into
// memory. TSV/CSV inputs are converted to canonical SQLite by the sidecar; a
// SQLite upload is validated and copied as-is.
func (h *ProjectHandler) importTFTarget(w http.ResponseWriter, r *http.Request, s *projectSession) {
	s.mu.Lock()
	busy := s.busy
	s.mu.Unlock()
	if busy {
		http.Error(w, "wait for the running step before replacing data", http.StatusConflict)
		return
	}

	tmp, filename, err := receiveTFUpload(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer os.Remove(tmp)

	lower := strings.ToLower(filename)
	dbPath := tmp
	bin, binErr := regutrace.Resolve()
	switch {
	case strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite"):
		if err := validateSQLiteFile(tmp); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if binErr != nil {
			http.Error(w, "ReguTrace binary is not available; cannot validate the prior", http.StatusServiceUnavailable)
			return
		}
		if err := validateSQLitePrior(r.Context(), bin, tmp); err != nil {
			http.Error(w, "invalid TF-target prior: "+err.Error(), http.StatusBadRequest)
			return
		}
	case strings.HasSuffix(lower, ".tsv") || strings.HasSuffix(lower, ".csv") ||
		strings.HasSuffix(lower, ".tsv.gz") || strings.HasSuffix(lower, ".csv.gz"):
		if binErr != nil {
			http.Error(w, "ReguTrace binary is not available; cannot convert the prior to SQLite", http.StatusServiceUnavailable)
			return
		}
		dbPath = tmp + ".db"
		defer os.Remove(dbPath)
		cmd := exec.CommandContext(r.Context(), bin,
			"convert", "--input", tmp, "--out", dbPath, "--format", "sqlite")
		subproc.HideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(dbPath)
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			http.Error(w, "convert TF-target prior: "+msg, http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "TF-target prior must be .db/.sqlite, .tsv/.csv, or .tsv.gz/.csv.gz", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		http.Error(w, "wait for the running step before replacing data", http.StatusConflict)
		return
	}
	f, err := os.Open(dbPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()
	if _, err = s.p.ImportFile("tf_target", "tf_target.db", f); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.validity = s.p.Validity(h.engine)
	h.reply(w, s, http.StatusOK)
}

func receiveTFUpload(w http.ResponseWriter, r *http.Request) (string, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<30)
	mr, err := r.MultipartReader()
	if err != nil {
		return "", "", fmt.Errorf("multipart upload required: %w", err)
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return "", "", errors.New("file field is required")
		}
		if err != nil {
			return "", "", err
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		filename := filepath.Base(part.FileName())
		if filename == "." || filename == "" {
			_ = part.Close()
			return "", "", errors.New("file name is required")
		}
		lower := strings.ToLower(filename)
		suffix := ""
		for _, candidate := range []string{".tsv.gz", ".csv.gz", ".tsv", ".csv", ".sqlite", ".db"} {
			if strings.HasSuffix(lower, candidate) {
				suffix = candidate
				break
			}
		}
		f, err := os.CreateTemp("", "rswgcna-tf-*"+suffix)
		if err != nil {
			_ = part.Close()
			return "", "", err
		}
		name := f.Name()
		_, copyErr := io.Copy(f, part)
		closeErr := f.Close()
		_ = part.Close()
		if copyErr != nil {
			_ = os.Remove(name)
			return "", "", copyErr
		}
		if closeErr != nil {
			_ = os.Remove(name)
			return "", "", closeErr
		}
		return name, filename, nil
	}
}

type tfTargetUpstreamRow struct {
	TFID           string   `json:"tf_id"`
	TFSymbol       string   `json:"tf_symbol"`
	ModuleTargets  int      `json:"module_targets"`
	NSites         int      `json:"n_sites"`
	BestP          *float64 `json:"best_p"`
	EvidenceLevels string   `json:"evidence_levels,omitempty"`
}

// tfTargetUpstream aggregates the imported prior over a module gene list.
// The prior remains an optional project auxiliary input; this endpoint does not
// run or invalidate the WGCNA worker.
func (h *ProjectHandler) tfTargetUpstream(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	var req struct {
		Genes []string `json:"genes"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid gene list", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid gene list", http.StatusBadRequest)
		return
	}
	seen := map[string]struct{}{}
	genes := make([]string, 0, len(req.Genes))
	for _, gene := range req.Genes {
		gene = strings.TrimSpace(gene)
		if gene == "" {
			continue
		}
		if strings.ContainsAny(gene, "\r\n") {
			http.Error(w, "gene IDs must not contain line breaks", http.StatusBadRequest)
			return
		}
		if _, ok := seen[gene]; ok {
			continue
		}
		seen[gene] = struct{}{}
		genes = append(genes, gene)
	}
	if len(genes) == 0 || len(genes) > 50000 {
		http.Error(w, "gene list must contain 1–50000 unique genes", http.StatusBadRequest)
		return
	}
	dbPath, err := s.p.InputPath("tf_target")
	if err != nil {
		http.Error(w, "TF-target prior not imported", http.StatusNotFound)
		return
	}
	bin, err := regutrace.Resolve()
	if err != nil {
		http.Error(w, "ReguTrace binary is not available", http.StatusServiceUnavailable)
		return
	}
	tmp, err := os.MkdirTemp("", "rswgcna-prior-query-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmp)
	targetFile := filepath.Join(tmp, "targets.txt")
	outFile := filepath.Join(tmp, "upstream.tsv")
	if err := os.WriteFile(targetFile, []byte(strings.Join(genes, "\n")+"\n"), 0o600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd := exec.CommandContext(r.Context(), bin,
		"prior", "--db", dbPath, "--targets", targetFile, "--out", outFile)
	subproc.HideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		http.Error(w, "query TF-target prior: "+msg, http.StatusBadRequest)
		return
	}
	f, err := os.Open(outFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	rdr := csv.NewReader(f)
	rdr.Comma = '\t'
	header, err := rdr.Read()
	if err != nil {
		http.Error(w, "read prior query result: "+err.Error(), http.StatusInternalServerError)
		return
	}
	col := func(name string) int {
		for i, h := range header {
			if h == name {
				return i
			}
		}
		return -1
	}
	idx := map[string]int{
		"tf_id": col("tf_id"), "tf_symbol": col("tf_symbol"),
		"module_targets": col("module_targets"), "n_sites": col("n_sites"),
		"best_p": col("best_p"), "evidence_levels": col("evidence_levels"),
	}
	for name, i := range idx {
		if i < 0 {
			http.Error(w, "prior query result missing column "+name, http.StatusInternalServerError)
			return
		}
	}
	rows := []tfTargetUpstreamRow{}
	for {
		rec, err := rdr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			http.Error(w, "read prior query result: "+err.Error(), http.StatusInternalServerError)
			return
		}
		dataRow := len(rows) + 1
		moduleTargets, err := parsePriorCount(rec[idx["module_targets"]], "module_targets", dataRow)
		if err != nil {
			http.Error(w, "read prior query result: "+err.Error(), http.StatusBadGateway)
			return
		}
		nSites, err := parsePriorCount(rec[idx["n_sites"]], "n_sites", dataRow)
		if err != nil {
			http.Error(w, "read prior query result: "+err.Error(), http.StatusBadGateway)
			return
		}
		bestP, err := parsePriorProbability(rec[idx["best_p"]], dataRow)
		if err != nil {
			http.Error(w, "read prior query result: "+err.Error(), http.StatusBadGateway)
			return
		}
		rows = append(rows, tfTargetUpstreamRow{
			TFID:           rec[idx["tf_id"]],
			TFSymbol:       rec[idx["tf_symbol"]],
			ModuleTargets:  moduleTargets,
			NSites:         nSites,
			BestP:          bestP,
			EvidenceLevels: rec[idx["evidence_levels"]],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"gene_count": len(genes),
		"rows":       rows,
	})
}

func parsePriorCount(value, column string, row int) (int, error) {
	value = strings.TrimSpace(value)
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("row %d: invalid %s %q", row, column, value)
	}
	return n, nil
}

func parsePriorProbability(value string, row int) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	p, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
		return nil, fmt.Errorf("row %d: invalid best_p %q", row, value)
	}
	return &p, nil
}

func validateSQLitePrior(ctx context.Context, bin, path string) error {
	cmd := exec.CommandContext(ctx, bin, "validate-db", "--db", path)
	subproc.HideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

func validateSQLiteFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var hdr [16]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return errors.New("not a SQLite database")
	}
	if string(hdr[:]) != "SQLite format 3\x00" {
		return errors.New("not a SQLite database")
	}
	return nil
}
