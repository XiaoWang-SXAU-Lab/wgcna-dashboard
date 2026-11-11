// Package regutrace runs the local ReguTrace TF-target scanner as a dashboard
// toolkit job and exposes its evidence table for querying and visualization.
package regutrace

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// Params are the scanner parameters exposed by the toolkit sidebar.
type Params struct {
	PromoterLen int        `json:"promoter_len"`
	Downstream  int        `json:"downstream"`
	PValue      float64    `json:"pvalue"`
	Background  [4]float64 `json:"background"`
	Threads     int        `json:"threads"` // 0 = auto (CPU cores - 1), 1 = serial
}

func defaultParams() Params {
	return Params{
		PromoterLen: 1000,
		Downstream:  0,
		PValue:      1e-4,
		Background:  [4]float64{0.25, 0.25, 0.25, 0.25},
		Threads:     0,
	}
}

func normalizeParams(p Params) (Params, error) {
	if p.PromoterLen == 0 {
		p.PromoterLen = 1000
	}
	if p.PValue == 0 {
		p.PValue = 1e-4
	}
	if p.Background == [4]float64{} {
		p.Background = [4]float64{0.25, 0.25, 0.25, 0.25}
	}
	if p.PromoterLen < 1 || p.PromoterLen > 1_000_000 {
		return p, errors.New("promoter_len must be between 1 and 1,000,000")
	}
	if p.Downstream < 0 || p.Downstream > 1_000_000 {
		return p, errors.New("downstream must be between 0 and 1,000,000")
	}
	if p.PValue < 0 || p.PValue > 1 {
		return p, errors.New("pvalue must be in [0, 1]")
	}
	if p.Threads < 0 || p.Threads > 256 {
		return p, errors.New("threads must be 0 (auto) or between 1 and 256")
	}
	sum := 0.0
	for _, v := range p.Background {
		if v <= 0 {
			return p, errors.New("background must contain four positive probabilities")
		}
		sum += v
	}
	if sum <= 0 {
		return p, errors.New("background sum must be positive")
	}
	return p, nil
}

// InputSummary is the safe, path-free view of the uploaded files.
type InputSummary struct {
	Genome           string `json:"genome"`
	GFF              string `json:"gff"`
	Motifs           string `json:"motifs"`
	MotifLibraryKey  string `json:"motif_library_key,omitempty"`
	TFMap            string `json:"tf_map,omitempty"`
	Annotation       string `json:"annotation,omitempty"`
	AnnotationSource string `json:"annotation_source"`
	GenomeBytes      int64  `json:"genome_bytes"`
	GFFBytes         int64  `json:"gff_bytes"`
}

// Job is the client-facing state of one ReguTrace run.
type Job struct {
	ID          string       `json:"id"`
	Status      string       `json:"status"` // uploaded | validating | validated | running | succeeded | failed | cancelled
	Stage       string       `json:"stage"`
	Message     string       `json:"message"`
	Progress    float64      `json:"progress"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Params      Params       `json:"params"`
	Inputs      InputSummary `json:"inputs"`
	Diagnostics *Report      `json:"diagnostics,omitempty"`
	Summary     *Summary     `json:"summary,omitempty"`
}

// Upload is one optional multipart file. Reader is consumed synchronously by
// Create.
type Upload struct {
	Name   string
	Size   int64
	Reader io.Reader
}

type Uploads struct {
	Genome       *Upload
	GFF          *Upload
	Motifs       *Upload
	MotifLibrary string // bundled JASPAR key; ignored when Motifs is present
	TFMap        *Upload
	Annotation   *Upload
}

type jobState struct {
	Job
	dir              string
	cancel           context.CancelFunc
	genomePath       string
	gffPath          string
	motifsPath       string
	tfMapPath        string
	annotationPath   string
	annotationSource string
	data             *dataset
	edges            []Edge
	promoters        map[string]*Promoter
	motifs           map[string]MotifInfo
	annotations      map[string]AnnotationEntry
}

type Manager struct {
	dir string
	bin string

	mu        sync.RWMutex
	jobs      map[string]*jobState
	libraries []LibraryInfo
}

// NewManager creates a ReguTrace job manager. bin may be empty in a build that
// does not embed the scanner; jobs then fail with an actionable error.
func NewManager(dir, bin string) *Manager {
	_ = os.MkdirAll(dir, 0o755)
	// Jobs are in-memory only. Keep the newest few on-disk directories for
	// post-mortem inspection, but remove older runs so repeated whole-genome
	// scans cannot fill the cache indefinitely.
	if entries, err := os.ReadDir(dir); err == nil {
		type oldJob struct {
			name    string
			modTime time.Time
		}
		var dirs []oldJob
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if info, err := entry.Info(); err == nil {
				dirs = append(dirs, oldJob{name: entry.Name(), modTime: info.ModTime()})
			}
		}
		sort.Slice(dirs, func(i, k int) bool { return dirs[i].modTime.After(dirs[k].modTime) })
		for i := 5; i < len(dirs); i++ {
			_ = os.RemoveAll(filepath.Join(dir, dirs[i].name))
		}
	}
	libraries := make([]LibraryInfo, 0, len(BuiltinLibraries))
	for _, lib := range BuiltinLibraries {
		meta, _ := parseMotifMeta(lib.Data)
		lib.Count = len(meta)
		libraries = append(libraries, LibraryInfo{Key: lib.Key, Name: lib.Name, Count: lib.Count})
	}
	return &Manager{dir: dir, bin: bin, jobs: map[string]*jobState{}, libraries: libraries}
}

// LibraryInfo describes one bundled JASPAR taxon collection.
type LibraryInfo struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Info describes the bundled motif libraries.
type Info struct {
	MotifLibrary string        `json:"motif_library"`
	MotifCount   int           `json:"motif_count"`
	Libraries    []LibraryInfo `json:"libraries"`
}

func (m *Manager) Info() Info {
	info := Info{Libraries: m.libraries}
	for _, lib := range m.libraries {
		if lib.Key == DefaultMotifLibrary {
			info.MotifLibrary = lib.Name
			info.MotifCount = lib.Count
			break
		}
	}
	if info.MotifLibrary == "" && len(m.libraries) > 0 {
		info.MotifLibrary = m.libraries[0].Name
		info.MotifCount = m.libraries[0].Count
	}
	return info
}

// Dir returns the job root (useful for diagnostics/tests).
func (m *Manager) Dir() string { return m.dir }

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

func saveUpload(dir, base string, up *Upload) (string, int64, error) {
	if up == nil || up.Reader == nil {
		return "", 0, errors.New("missing upload")
	}
	br := bufio.NewReader(up.Reader)
	magic, _ := br.Peek(2)
	path := filepath.Join(dir, base)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		path += ".gz"
	}
	f, err := os.Create(path)
	if err != nil {
		return "", 0, err
	}
	n, copyErr := io.Copy(f, br)
	closeErr := f.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	if n == 0 {
		return "", 0, fmt.Errorf("uploaded %s is empty", base)
	}
	return path, n, nil
}

// Create stores the uploaded inputs and returns a new job in "uploaded" state.
func (m *Manager) Create(ups Uploads) (Job, error) {
	if ups.Genome == nil || ups.GFF == nil {
		return Job{}, errors.New("genome FASTA and GFF3 are required")
	}
	id := newID()
	dir := filepath.Join(m.dir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Job{}, err
	}
	cleanup := func(err error) (Job, error) {
		_ = os.RemoveAll(dir)
		return Job{}, err
	}

	genomePath, genomeSize, err := saveUpload(dir, "genome.fa", ups.Genome)
	if err != nil {
		return cleanup(fmt.Errorf("save genome: %w", err))
	}
	gffPath, gffSize, err := saveUpload(dir, "genome.gff3", ups.GFF)
	if err != nil {
		return cleanup(fmt.Errorf("save GFF3: %w", err))
	}

	libraryKey := ups.MotifLibrary
	if libraryKey == "" {
		libraryKey = DefaultMotifLibrary
	}
	motifsDisplay := ""
	motifLibraryKey := ""
	var motifsPath string
	var motifBytes []byte
	if ups.Motifs != nil {
		motifsPath, _, err = saveUpload(dir, "motifs.jaspar", ups.Motifs)
		if err != nil {
			return cleanup(fmt.Errorf("save motif library: %w", err))
		}
		motifsDisplay = filepath.Base(ups.Motifs.Name)
		motifLibraryKey = "custom"
		motifBytes, err = readMaybeGzip(motifsPath)
		if err != nil {
			return cleanup(fmt.Errorf("read motif library: %w", err))
		}
	} else {
		lib, ok := builtinLibrary(libraryKey)
		if !ok {
			return cleanup(fmt.Errorf("unknown motif library %q; choose a bundled JASPAR taxon or upload a JASPAR PFM", libraryKey))
		}
		motifsDisplay = lib.Name
		motifLibraryKey = lib.Key
		motifsPath = filepath.Join(dir, "motifs.jaspar")
		motifBytes = lib.Data
		if err := os.WriteFile(motifsPath, motifBytes, 0o644); err != nil {
			return cleanup(fmt.Errorf("write bundled motif library: %w", err))
		}
	}
	if len(motifBytes) > 128<<20 {
		return cleanup(errors.New("motif library is larger than 128 MB"))
	}
	motifs, err := parseMotifMeta(motifBytes)
	if err != nil {
		return cleanup(fmt.Errorf("motif library: %w", err))
	}

	tfMapPath := ""
	tfMapDisplay := ""
	if ups.TFMap != nil {
		tfMapPath, _, err = saveUpload(dir, "tf_map.tsv", ups.TFMap)
		if err != nil {
			return cleanup(fmt.Errorf("save TF map: %w", err))
		}
		tfMapDisplay = filepath.Base(ups.TFMap.Name)
	}

	annotations := map[string]AnnotationEntry{}
	annotationSource := "GFF"
	annotationPath := ""
	annotationDisplay := ""
	if ups.Annotation != nil {
		raw, err := io.ReadAll(io.LimitReader(ups.Annotation.Reader, 512<<20+1))
		if err != nil {
			return cleanup(fmt.Errorf("read annotation: %w", err))
		}
		if len(raw) > 512<<20 {
			return cleanup(errors.New("annotation table is larger than 512 MB"))
		}
		if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
			gz, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				return cleanup(fmt.Errorf("annotation gzip: %w", err))
			}
			raw, err = io.ReadAll(io.LimitReader(gz, 512<<20+1))
			_ = gz.Close()
			if err != nil {
				return cleanup(fmt.Errorf("read gzipped annotation: %w", err))
			}
			if len(raw) > 512<<20 {
				return cleanup(errors.New("annotation table is larger than 512 MB after decompression"))
			}
		}
		var normalized []byte
		annotations, normalized, annotationSource, err = parseAnnotation(raw)
		if err != nil {
			return cleanup(err)
		}
		annotationPath = filepath.Join(dir, "annotation.tsv")
		if err := os.WriteFile(annotationPath, normalized, 0o644); err != nil {
			return cleanup(fmt.Errorf("write normalized annotation: %w", err))
		}
		annotationDisplay = filepath.Base(ups.Annotation.Name)
	}

	now := time.Now()
	js := &jobState{
		Job: Job{
			ID:        id,
			Status:    "uploaded",
			Stage:     "uploaded",
			Message:   "inputs uploaded; validate or run",
			CreatedAt: now,
			UpdatedAt: now,
			Params:    defaultParams(),
			Inputs: InputSummary{
				Genome:           filepath.Base(ups.Genome.Name),
				GFF:              filepath.Base(ups.GFF.Name),
				Motifs:           motifsDisplay,
				MotifLibraryKey:  motifLibraryKey,
				TFMap:            tfMapDisplay,
				Annotation:       annotationDisplay,
				AnnotationSource: annotationSource,
				GenomeBytes:      genomeSize,
				GFFBytes:         gffSize,
			},
		},
		dir:              dir,
		genomePath:       genomePath,
		gffPath:          gffPath,
		motifsPath:       motifsPath,
		tfMapPath:        tfMapPath,
		annotationPath:   annotationPath,
		annotationSource: annotationSource,
		promoters:        map[string]*Promoter{},
		motifs:           motifs,
		annotations:      annotations,
	}
	m.mu.Lock()
	m.jobs[id] = js
	m.pruneLocked(5)
	m.mu.Unlock()
	return js.snapshot(), nil
}

// pruneLocked keeps the newest jobs and removes older evidence directories.
// Whole-genome ReguTrace runs can produce multi-gigabyte TSV files, so an
// unbounded cache would otherwise fill the machine's disk.
func (m *Manager) pruneLocked(keep int) {
	if len(m.jobs) <= keep {
		return
	}
	type entry struct {
		id  string
		job *jobState
	}
	entries := make([]entry, 0, len(m.jobs))
	for id, js := range m.jobs {
		entries = append(entries, entry{id: id, job: js})
	}
	sort.Slice(entries, func(i, k int) bool {
		return entries[i].job.CreatedAt.After(entries[k].job.CreatedAt)
	})
	for i := keep; i < len(entries); i++ {
		js := entries[i].job
		if js.cancel != nil || js.Status == "validating" || js.Status == "running" {
			continue
		}
		_ = os.RemoveAll(js.dir)
		delete(m.jobs, entries[i].id)
	}
}

// Start launches validate or run asynchronously. mode is "validate" or "run".
func (m *Manager) Start(id string, params Params, mode string) (Job, error) {
	if mode != "validate" && mode != "run" {
		return Job{}, fmt.Errorf("unknown mode %q", mode)
	}
	if m.bin == "" {
		return Job{}, errors.New("ReguTrace binary is not available; set WGCNA_REGUTRACE or build with -tags release")
	}
	params, err := normalizeParams(params)
	if err != nil {
		return Job{}, err
	}
	m.mu.Lock()
	js, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return Job{}, errors.New("job not found")
	}
	if js.Status == "validating" || js.Status == "running" {
		m.mu.Unlock()
		return Job{}, errors.New("job is already running")
	}
	js.Params = params
	js.Error = ""
	js.Diagnostics = nil
	js.Progress = 0
	js.UpdatedAt = time.Now()
	js.Summary = nil
	js.data = nil
	js.edges = nil
	js.promoters = map[string]*Promoter{}
	if mode == "validate" {
		js.Status = "validating"
		js.Stage = "validating"
		js.Message = "validating inputs"
	} else {
		js.Status = "running"
		js.Stage = "starting"
		js.Message = "starting scanner"
	}
	ctx, cancel := context.WithCancel(context.Background())
	js.cancel = cancel
	snap := js.snapshotLocked()
	m.mu.Unlock()

	go m.execute(ctx, js, mode)
	return snap, nil
}

func (m *Manager) execute(ctx context.Context, js *jobState, mode string) {
	defer func() {
		m.mu.Lock()
		js.cancel = nil
		m.mu.Unlock()
	}()
	args := m.buildArgs(js, mode)
	cmd := exec.CommandContext(ctx, m.bin, args...)
	subproc.HideWindow(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		m.fail(js, fmt.Sprintf("start ReguTrace: %v", err))
		return
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			<-done
			_ = os.WriteFile(filepath.Join(js.dir, "run.log"), output.Bytes(), 0o644)
			m.mu.Lock()
			js.Status = "cancelled"
			js.Stage = "cancelled"
			js.Message = "cancelled"
			js.Progress = 0
			js.UpdatedAt = time.Now()
			m.mu.Unlock()
			return
		case err := <-done:
			_ = os.WriteFile(filepath.Join(js.dir, "run.log"), output.Bytes(), 0o644)
			m.finish(js, mode, err, output.String())
			return
		case <-ticker.C:
			if p, err := readProgress(filepath.Join(js.dir, "progress.json")); err == nil {
				m.mu.Lock()
				js.Stage = p.Stage
				js.Progress = overallProgress(p.Stage, p.Progress)
				js.Message = p.Message
				js.UpdatedAt = time.Now()
				m.mu.Unlock()
			}
		}
	}
}

func (m *Manager) buildArgs(js *jobState, mode string) []string {
	args := []string{mode}
	if mode == "run" {
		args[0] = "scan"
	}
	args = append(args,
		"--genome", js.genomePath,
		"--gff", js.gffPath,
		"--motifs", js.motifsPath,
		"--promoter-len", strconv.Itoa(js.Params.PromoterLen),
		"--downstream", strconv.Itoa(js.Params.Downstream),
		"--pvalue", strconv.FormatFloat(js.Params.PValue, 'g', -1, 64),
		"--background", fmt.Sprintf("%g,%g,%g,%g", js.Params.Background[0], js.Params.Background[1], js.Params.Background[2], js.Params.Background[3]),
		"--report-json", filepath.Join(js.dir, "report.json"),
	)
	if js.tfMapPath != "" {
		args = append(args, "--tf-map", js.tfMapPath)
	}
	if js.annotationPath != "" {
		args = append(args, "--annotation", js.annotationPath)
	}
	if js.Params.Threads > 0 {
		args = append(args, "--threads", strconv.Itoa(js.Params.Threads))
	}
	if mode == "run" {
		args = append(args,
			"--out", filepath.Join(js.dir, "evidence.tsv"),
			"--promoters-out", filepath.Join(js.dir, "promoters.fa"),
			"--progress-json", filepath.Join(js.dir, "progress.json"),
			"--stream",
		)
	} else {
		args = append(args, "--progress-json", filepath.Join(js.dir, "progress.json"))
	}
	return args
}

func (m *Manager) finish(js *jobState, mode string, runErr error, logText string) {
	report, reportErr := readReport(filepath.Join(js.dir, "report.json"))
	if reportErr == nil {
		sanitizeReport(report, js.dir)
		if b, err := json.MarshalIndent(report, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(js.dir, "report.json"), b, 0o644)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	js.UpdatedAt = time.Now()
	if reportErr == nil {
		js.Diagnostics = report
	}
	if runErr != nil {
		msg := sanitizeText(strings.TrimSpace(logText), js.dir)
		if len(msg) > 4000 {
			msg = msg[len(msg)-4000:]
		}
		if report != nil && report.Summary.Errors > 0 {
			msg = fmt.Sprintf("input validation reported %d error(s)", report.Summary.Errors)
		}
		if msg == "" {
			msg = runErr.Error()
		}
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "failed"
		js.Error = msg
		js.Progress = 0
		return
	}
	if reportErr != nil {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "missing diagnostics report"
		js.Error = reportErr.Error()
		js.Progress = 0
		return
	}
	if report != nil && report.Summary.Errors > 0 {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "input validation failed"
		js.Error = fmt.Sprintf("input validation reported %d error(s)", report.Summary.Errors)
		js.Progress = 0
		return
	}
	if mode == "validate" {
		js.Status = "validated"
		js.Stage = "validated"
		js.Message = "inputs validated"
		js.Progress = 1
		return
	}
	promoters, err := readPromoters(filepath.Join(js.dir, "promoters.fa"))
	if err != nil {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "failed to read promoters"
		js.Error = err.Error()
		return
	}
	data, err := buildDataset(
		filepath.Join(js.dir, "evidence.tsv"),
		js.motifs,
		js.annotations,
		len(promoters),
		js.annotationSource,
		reportContextInt(report, "SCAN_SUMMARY", "motif_hits"),
	)
	if err != nil {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "failed to index evidence"
		js.Error = err.Error()
		return
	}
	data.summary.Threads = reportContextInt(report, "SCAN_SUMMARY", "threads")
	js.edges = nil
	js.data = data
	js.promoters = promoters
	js.Summary = data.summary
	js.Status = "succeeded"
	js.Stage = "succeeded"
	js.Message = fmt.Sprintf("%d evidence rows", data.total)
	js.Progress = 1
}

func (m *Manager) fail(js *jobState, msg string) {
	m.mu.Lock()
	js.Status = "failed"
	js.Stage = "failed"
	js.Message = "failed"
	js.Error = msg
	js.UpdatedAt = time.Now()
	m.mu.Unlock()
}

// Get returns a snapshot of one job.
func (m *Manager) Get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	snap := js.snapshotLocked()
	if js.Status == "running" || js.Status == "validating" {
		if p, err := readProgress(filepath.Join(js.dir, "progress.json")); err == nil {
			snap.Stage = p.Stage
			snap.Progress = overallProgress(p.Stage, p.Progress)
			snap.Message = p.Message
		}
	}
	return snap, true
}

// Cancel stops a running job.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	js, ok := m.jobs[id]
	if !ok {
		return errors.New("job not found")
	}
	if js.cancel == nil {
		return errors.New("job is not running")
	}
	js.cancel()
	return nil
}

// Query runs a free-text TF/target query over the completed evidence table.
func (m *Manager) Query(id string, opts QueryOptions) (QueryResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return QueryResult{}, errors.New("job not found")
	}
	if js.Status != "succeeded" {
		return QueryResult{}, errors.New("job has not completed a scan")
	}
	return js.query(opts), nil
}

// PromoterView returns one target's promoter and grouped binding sites.
func (m *Manager) PromoterView(id, target string) (*PromoterView, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return nil, errors.New("job not found")
	}
	if js.Status != "succeeded" {
		return nil, errors.New("job has not completed a scan")
	}
	return js.promoterView(target)
}

// ExportPath returns an on-disk artifact and a download filename.
func (m *Manager) ExportPath(id, kind string) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return "", "", errors.New("job not found")
	}
	name := ""
	switch kind {
	case "evidence":
		name = "evidence.tsv"
	case "report", "diagnostics":
		name = "report.json"
	case "promoters":
		name = "promoters.fa"
	case "log":
		name = "run.log"
	default:
		return "", "", errors.New("unknown export kind")
	}
	path := filepath.Join(js.dir, name)
	if _, err := os.Stat(path); err != nil {
		return "", "", err
	}
	return path, "regutrace-" + id + "-" + name, nil
}

func (j *jobState) snapshot() Job {
	return j.snapshotLocked()
}

func (j *jobState) snapshotLocked() Job {
	c := j.Job
	if j.Diagnostics != nil {
		r := *j.Diagnostics
		c.Diagnostics = &r
	}
	if j.Summary != nil {
		s := *j.Summary
		c.Summary = &s
	}
	return c
}

func reportContextInt(r *Report, code, key string) int {
	if r == nil {
		return 0
	}
	for _, d := range r.Diagnostics {
		if d.Code != code {
			continue
		}
		raw := d.Context[key]
		if raw == "" {
			return 0
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

func sanitizeText(s, dir string) string {
	if dir == "" {
		return s
	}
	s = strings.ReplaceAll(s, dir+string(os.PathSeparator), "")
	s = strings.ReplaceAll(s, dir, "")
	return s
}

func sanitizeReport(r *Report, dir string) {
	if r == nil {
		return
	}
	for k, v := range r.Inputs {
		r.Inputs[k] = sanitizeText(v, dir)
	}
	for i := range r.Diagnostics {
		r.Diagnostics[i].File = sanitizeText(r.Diagnostics[i].File, dir)
		r.Diagnostics[i].Message = sanitizeText(r.Diagnostics[i].Message, dir)
		r.Diagnostics[i].Suggestion = sanitizeText(r.Diagnostics[i].Suggestion, dir)
		for k, v := range r.Diagnostics[i].Context {
			r.Diagnostics[i].Context[k] = sanitizeText(v, dir)
		}
	}
}

type liveProgress struct {
	Stage    string  `json:"stage"`
	Status   string  `json:"status"`
	Done     uint64  `json:"done"`
	Total    uint64  `json:"total"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

func readProgress(path string) (*liveProgress, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p liveProgress
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func overallProgress(stage string, local float64) float64 {
	if local < 0 {
		local = 0
	}
	if local > 1 {
		local = 1
	}
	switch stage {
	case "starting":
		return 0.01
	case "reading_motifs":
		return 0.02 + 0.02*local
	case "reading_genome":
		return 0.04 + 0.04*local
	case "reading_gff":
		return 0.08 + 0.04*local
	case "extracting_promoters":
		return 0.12 + 0.08*local
	case "writing_promoters":
		return 0.20 + 0.03*local
	case "scanning":
		return 0.23 + 0.71*local
	case "writing_evidence":
		return 0.94 + 0.05*local
	case "done", "succeeded":
		return 1
	default:
		return local * 0.2
	}
}
