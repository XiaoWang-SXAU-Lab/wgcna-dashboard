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
	PromoterLen                 int        `json:"promoter_len"`
	Downstream                  int        `json:"downstream"`
	PValue                      float64    `json:"pvalue"`
	Background                  [4]float64 `json:"background"`
	BackgroundMode              string     `json:"background_mode,omitempty"`
	Threads                     int        `json:"threads"` // 0 = auto (CPU cores - 1), 1 = serial
	IncludeSingleDomainFallback bool       `json:"include_single_domain_fallback"`
	ReuseExactPWM               bool       `json:"reuse_exact_pwm,omitempty"`
	Phase3Artifacts             bool       `json:"phase3_artifacts,omitempty"`
	UseScanCache                bool       `json:"use_scan_cache,omitempty"`
	SitesOnly                   bool       `json:"sites_only,omitempty"`
	MappingMode                 string     `json:"mapping_mode,omitempty"`
}

func defaultParams() Params {
	return Params{
		PromoterLen: 1000,
		Downstream:  0,
		PValue:      1e-4,
		Background:  [4]float64{0.25, 0.25, 0.25, 0.25},
		// The uniform i.i.d. null mis-calibrates every p-value on A/T-rich
		// genomes; the promoter A/C/G/T model is the default. `uniform` remains
		// selectable to reproduce jobs run before this default changed.
		BackgroundMode: "empirical",
		Threads:        0,
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
	if p.BackgroundMode == "" {
		p.BackgroundMode = "empirical"
	}
	if p.BackgroundMode != "uniform" && p.BackgroundMode != "empirical" {
		return p, errors.New("background_mode must be uniform or empirical")
	}
	if p.MappingMode == "" {
		p.MappingMode = "strict"
	}
	if p.MappingMode != "strict" && p.MappingMode != "balanced" && p.MappingMode != "exploratory" {
		return p, errors.New("mapping_mode must be strict, balanced or exploratory")
	}
	if p.SitesOnly && !p.Phase3Artifacts {
		return p, errors.New("sites_only requires phase3_artifacts")
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
	TFGenes          string `json:"tf_genes,omitempty"`
	CDS              string `json:"cds,omitempty"`
	Proteins         string `json:"proteins,omitempty"`
	HMM              string `json:"hmm,omitempty"`
	FamilyMap        string `json:"family_map,omitempty"`
	Trace            string `json:"trace,omitempty"`
	Annotation       string `json:"annotation,omitempty"`
	AnnotationSource string `json:"annotation_source"`
	GenomeBytes      int64  `json:"genome_bytes"`
	GFFBytes         int64  `json:"gff_bytes"`
	ReferencePack    string `json:"reference_pack,omitempty"`
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
	Genome        *Upload
	GFF           *Upload
	Motifs        *Upload
	MotifLibrary  string // bundled JASPAR key; ignored when Motifs is present
	TFMap         *Upload
	TFGenes       *Upload
	Proteins      *Upload
	CDS           *Upload
	HMM           *Upload
	FamilyMap     *Upload
	Trace         *Upload
	CustomPack    *Upload
	ReferencePack string
	Annotation    *Upload
}

type jobState struct {
	Job
	dir                 string
	generation          uint64
	evidencePath        string
	cancel              context.CancelFunc
	genomePath          string
	gffPath             string
	gff                 *gffCatalog
	motifsPath          string
	tfMapPath           string
	tfGenesPath         string
	proteinsPath        string
	sequenceIsCDS       bool
	hmmPath             string
	familyMapPath       string
	tracePath           string
	annotationPath      string
	annotationSource    string
	referencePackDir    string
	referencePackKey    string
	pipelineDir         string
	reportPath          string
	promotersPath       string
	progressPath        string
	sitesPath           string
	testabilityPath     string
	promoterSummaryPath string
	scanManifestPath    string
	pipelineReportPath  string
	data                *dataset
	edges               []Edge
	promoters           map[string]*Promoter
	motifs              map[string]MotifInfo
	annotations         map[string]AnnotationEntry
	donors              map[donorKey][]DonorInfo
	phase4              *phase4Metadata
	proteinSequences    map[string]string
	proteinMu           sync.Mutex
	checksums           map[string]string
	checksumMu          sync.Mutex
	tfTargetCache       map[string]TFTargetResult
	tfTargetOrder       []string
	tfTargetMu          sync.Mutex
}

type Manager struct {
	dir        string
	bin        string
	packRoot   string
	toolRoot   string
	toolStatus []ToolStatus
	toolFiles  map[string]string
	toolErr    error

	mu        sync.RWMutex
	exportMu  sync.Mutex
	jobs      map[string]*jobState
	libraries []LibraryInfo
}

// NewManager creates a ReguTrace job manager. bin may be empty in a build that
// does not embed the scanner; jobs then fail with an actionable error.
func NewManager(dir, bin string, packRoots ...string) *Manager {
	_ = os.MkdirAll(dir, 0o755)
	packRoot := ""
	if len(packRoots) > 0 {
		packRoot = packRoots[0]
	} else {
		packRoot = os.Getenv("WGCNA_REGUTRACE_PACKS")
		if packRoot == "" {
			if config, err := os.UserConfigDir(); err == nil {
				packRoot = filepath.Join(config, "rsWGCNA-explore", "regutrace-packs")
			}
		}
	}
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
	return &Manager{dir: dir, bin: bin, packRoot: packRoot, jobs: map[string]*jobState{}, libraries: libraries}
}

// LibraryInfo describes one bundled JASPAR taxon collection.
type LibraryInfo struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Info describes the bundled motif libraries.
type Info struct {
	MotifLibrary string              `json:"motif_library"`
	MotifCount   int                 `json:"motif_count"`
	Libraries    []LibraryInfo       `json:"libraries"`
	Packs        []ReferencePackInfo `json:"packs"`
	Tools        []ToolStatus        `json:"tools,omitempty"`
}

func (m *Manager) Info() Info {
	info := Info{Libraries: m.libraries, Packs: ListReferencePacks(m.packRoot), Tools: m.toolStatus}
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

// SetToolRoot registers the installer-bundled tools directory and verifies its
// manifest before any pack job can run. An empty root keeps the developer
// path/PATH fallback.
func (m *Manager) SetToolRoot(root string) {
	m.toolRoot = strings.TrimSpace(root)
	m.toolStatus, m.toolFiles, m.toolErr = VerifyBundledTools(m.toolRoot)
}

func (m *Manager) toolPath(name string) string {
	switch name {
	case "hmmsearch":
		if value := strings.TrimSpace(os.Getenv("WGCNA_HMMERSEARCH")); value != "" {
			return value
		}
	case "blastp":
		if value := strings.TrimSpace(os.Getenv("WGCNA_BLASTP")); value != "" {
			return value
		}
	}
	return m.toolFiles[name]
}

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
	packMode := strings.TrimSpace(ups.ReferencePack) != "" || ups.CustomPack != nil
	var pack *referencePack
	if ups.CustomPack != nil {
		if strings.TrimSpace(ups.ReferencePack) != "" {
			return cleanup(errors.New("choose either a server reference pack or a custom pack ZIP, not both"))
		}
		pack, err = unpackCustomPack(dir, ups.CustomPack)
		if err != nil {
			return cleanup(err)
		}
	} else if packMode {
		pack, err = ResolveReferencePack(m.packRoot, ups.ReferencePack)
		if err != nil {
			return cleanup(err)
		}
	}

	motifsDisplay := ""
	motifLibraryKey := ""
	var motifsPath string
	var motifBytes []byte
	if packMode {
		if ups.Motifs != nil || strings.TrimSpace(ups.MotifLibrary) != "" {
			return cleanup(errors.New("a reference pack supplies motifs; do not upload or select a motif library in pack mode"))
		}
		motifsPath = pack.MotifsPath
		motifBytes, err = readMaybeGzip(motifsPath)
		if err != nil {
			return cleanup(fmt.Errorf("read pack motif library: %w", err))
		}
		motifsDisplay = filepath.Base(pack.manifest.Files["motifs"])
		motifLibraryKey = "pack:" + pack.Info.Key
	} else {
		libraryKey := ups.MotifLibrary
		if libraryKey == "" {
			libraryKey = DefaultMotifLibrary
		}
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
	tfGenesPath := ""
	tfGenesDisplay := ""
	proteinsPath, proteinsDisplay, cdsDisplay := "", "", ""
	hmmPath, hmmDisplay := "", ""
	familyMapPath, familyMapDisplay := "", ""
	tracePath, traceDisplay := "", ""
	if packMode {
		if ups.HMM != nil || ups.TFMap != nil || ups.TFGenes != nil || ups.FamilyMap != nil || ups.Trace != nil {
			return cleanup(errors.New("reference pack mode generates the TF catalogue, TF–motif map and trace table; do not upload HMM/tf_map/tf_genes/family_map/trace"))
		}
		if (ups.Proteins == nil) == (ups.CDS == nil) {
			return cleanup(errors.New("provide exactly one protein or CDS FASTA"))
		}
		upload, filename := ups.Proteins, "proteins.faa"
		if ups.CDS != nil {
			upload, filename = ups.CDS, "cds.fna"
			cdsDisplay = filepath.Base(upload.Name)
		} else {
			proteinsDisplay = filepath.Base(upload.Name)
		}
		proteinsPath, _, err = saveUpload(dir, filename, upload)
		if err != nil {
			return cleanup(fmt.Errorf("save TF sequences: %w", err))
		}
	} else {
		if ups.CDS != nil {
			return cleanup(errors.New("CDS FASTA requires a reference-backed analysis"))
		}
		if ups.TFGenes != nil && ups.TFMap == nil {
			return cleanup(errors.New("tf_genes requires tf_map; choose a mapped TF table"))
		}
		if ups.TFMap != nil {
			tfMapPath, _, err = saveUpload(dir, "tf_map.tsv", ups.TFMap)
			if err != nil {
				return cleanup(fmt.Errorf("save TF map: %w", err))
			}
			tfMapDisplay = filepath.Base(ups.TFMap.Name)
		}
		if ups.TFGenes != nil {
			tfGenesPath, _, err = saveUpload(dir, "tf_genes.tsv", ups.TFGenes)
			if err != nil {
				return cleanup(fmt.Errorf("save TF genes: %w", err))
			}
			tfGenesDisplay = filepath.Base(ups.TFGenes.Name)
		}
		if (ups.Proteins == nil) != (ups.HMM == nil) {
			return cleanup(errors.New("proteins and HMM must be supplied together"))
		}
		if ups.FamilyMap != nil && ups.Proteins == nil {
			return cleanup(errors.New("family_map requires proteins and HMM"))
		}
		if ups.Proteins != nil && tfMapPath == "" {
			return cleanup(errors.New("HMMER TF identification requires a tf_map.tsv"))
		}
		if ups.Proteins != nil {
			proteinsPath, _, err = saveUpload(dir, "proteins.faa", ups.Proteins)
			if err != nil {
				return cleanup(fmt.Errorf("save proteins: %w", err))
			}
			proteinsDisplay = filepath.Base(ups.Proteins.Name)
			hmmPath, _, err = saveUpload(dir, "tf.hmm", ups.HMM)
			if err != nil {
				return cleanup(fmt.Errorf("save HMM: %w", err))
			}
			hmmDisplay = filepath.Base(ups.HMM.Name)
			if ups.FamilyMap != nil {
				familyMapPath, _, err = saveUpload(dir, "hmm_family.tsv", ups.FamilyMap)
				if err != nil {
					return cleanup(fmt.Errorf("save family map: %w", err))
				}
				familyMapDisplay = filepath.Base(ups.FamilyMap.Name)
			}
		}
		if ups.Trace != nil {
			tracePath, _, err = saveUpload(dir, "tf_motif_trace.tsv", ups.Trace)
			if err != nil {
				return cleanup(fmt.Errorf("save trace table: %w", err))
			}
			traceDisplay = filepath.Base(ups.Trace.Name)
		}
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
		// EggNOG runs on protein/transcript IDs in many species (for example
		// Fta_1G0000020.1), while GFF targets use gene IDs (Fta_1G0000020).
		// Keep exact keys authoritative and add gene-level aliases for the UI.
		annotations = withVersionAliases(annotations)
		annotationPath = filepath.Join(dir, "annotation.tsv")
		if err := os.WriteFile(annotationPath, normalized, 0o644); err != nil {
			return cleanup(fmt.Errorf("write normalized annotation: %w", err))
		}
		annotationDisplay = filepath.Base(ups.Annotation.Name)
	}

	referencePackKey := ""
	referencePackDir := ""
	if packMode {
		referencePackKey = pack.Info.Key
		referencePackDir = pack.Dir
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
				TFGenes:          tfGenesDisplay,
				Proteins:         proteinsDisplay,
				CDS:              cdsDisplay,
				HMM:              hmmDisplay,
				FamilyMap:        familyMapDisplay,
				Trace:            traceDisplay,
				Annotation:       annotationDisplay,
				AnnotationSource: annotationSource,
				GenomeBytes:      genomeSize,
				GFFBytes:         gffSize,
				ReferencePack:    referencePackKey,
			},
		},
		dir:              dir,
		genomePath:       genomePath,
		gffPath:          gffPath,
		gff:              newGFFCatalog(gffPath),
		motifsPath:       motifsPath,
		tfMapPath:        tfMapPath,
		tfGenesPath:      tfGenesPath,
		proteinsPath:     proteinsPath,
		sequenceIsCDS:    ups.CDS != nil,
		hmmPath:          hmmPath,
		familyMapPath:    familyMapPath,
		tracePath:        tracePath,
		referencePackDir: referencePackDir,
		referencePackKey: referencePackKey,
		reportPath:       filepath.Join(dir, "report.json"),
		promotersPath:    filepath.Join(dir, "promoters.fa"),
		progressPath:     filepath.Join(dir, "progress.json"),
		annotationPath:   annotationPath,
		annotationSource: annotationSource,
		promoters:        map[string]*Promoter{},
		motifs:           motifs,
		annotations:      annotations,
		donors:           map[donorKey][]DonorInfo{},
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
		return Job{}, fmt.Errorf(
			"ReguTrace binary is not available; place %s next to the application or set WGCNA_REGUTRACE",
			binaryName(),
		)
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
	if js.referencePackDir != "" && mode == "validate" {
		m.mu.Unlock()
		return Job{}, errors.New("reference pack jobs run the full pipeline directly; use Run, not Validate")
	}
	if js.referencePackDir != "" && m.toolErr != nil {
		envH := strings.TrimSpace(os.Getenv("WGCNA_HMMERSEARCH")) != ""
		envB := strings.TrimSpace(os.Getenv("WGCNA_BLASTP")) != ""
		if !envH || !envB {
			m.mu.Unlock()
			return Job{}, fmt.Errorf("bundled tools failed verification: %w; set WGCNA_HMMERSEARCH/WGCNA_BLASTP only as an advanced override", m.toolErr)
		}
	}
	js.Params = params
	if mode == "run" {
		js.generation++
		if js.referencePackDir != "" {
			js.pipelineDir = filepath.Join(js.dir, fmt.Sprintf("pipeline.%d", js.generation))
			js.evidencePath = filepath.Join(js.pipelineDir, "evidence.tsv")
			js.reportPath = filepath.Join(js.pipelineDir, "scan-report.json")
			js.promotersPath = filepath.Join(js.pipelineDir, "promoters.fa")
			js.progressPath = filepath.Join(js.pipelineDir, "progress.json")
			js.tfGenesPath = filepath.Join(js.pipelineDir, "tf_genes.tsv")
			js.tfMapPath = filepath.Join(js.pipelineDir, "tf_map.tsv")
			js.tracePath = filepath.Join(js.pipelineDir, "tf_motif_trace.tsv")
			js.sitesPath = filepath.Join(js.pipelineDir, "motif-site-hits.tsv")
			js.testabilityPath = filepath.Join(js.pipelineDir, "motif-testability.tsv")
			js.promoterSummaryPath = filepath.Join(js.pipelineDir, "promoter-summary.tsv")
			js.scanManifestPath = filepath.Join(js.pipelineDir, "scan-manifest.json")
			js.pipelineReportPath = filepath.Join(js.pipelineDir, "pipeline-report.json")
		} else {
			js.evidencePath = filepath.Join(js.dir, fmt.Sprintf("evidence.%d.tsv", js.generation))
			js.reportPath = filepath.Join(js.dir, "report.json")
			js.promotersPath = filepath.Join(js.dir, "promoters.fa")
			js.progressPath = filepath.Join(js.dir, "progress.json")
			js.sitesPath = filepath.Join(js.dir, fmt.Sprintf("motif-site-hits.%d.tsv", js.generation))
			js.testabilityPath = filepath.Join(js.dir, fmt.Sprintf("motif-testability.%d.tsv", js.generation))
			js.promoterSummaryPath = filepath.Join(js.dir, fmt.Sprintf("promoter-summary.%d.tsv", js.generation))
			js.scanManifestPath = filepath.Join(js.dir, fmt.Sprintf("scan-manifest.%d.json", js.generation))
			js.pipelineReportPath = ""
		}
	}
	js.Error = ""
	js.Diagnostics = nil
	js.Progress = 0
	js.UpdatedAt = time.Now()
	js.Summary = nil
	js.data = nil
	js.edges = nil
	js.promoters = map[string]*Promoter{}
	js.donors = map[donorKey][]DonorInfo{}
	if mode == "validate" {
		js.Status = "validating"
		js.Stage = "validating"
		js.Message = "validating inputs"
	} else {
		js.Status = "running"
		js.Stage = "starting"
		if js.referencePackDir != "" {
			js.Message = "starting reference-pack pipeline"
		} else {
			js.Message = "starting scanner"
		}
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
	if js.proteinsPath != "" && js.tfGenesPath == "" {
		if err := m.identifyTFs(ctx, js); err != nil {
			m.fail(js, err.Error())
			return
		}
		m.mu.Lock()
		if mode == "validate" {
			js.Stage = "validating"
			js.Message = "validating inputs"
		} else {
			js.Stage = "starting"
			js.Message = "starting scanner"
		}
		js.UpdatedAt = time.Now()
		m.mu.Unlock()
	}
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
			if p, err := readProgress(js.progressPath); err == nil {
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

func (m *Manager) identifyTFs(ctx context.Context, js *jobState) error {
	m.mu.Lock()
	js.Stage = "identifying_tfs"
	js.Message = "identifying TFs from representative proteins"
	js.Progress = 0
	js.UpdatedAt = time.Now()
	proteins := js.proteinsPath
	hmm := js.hmmPath
	familyMap := js.familyMapPath
	dir := js.dir
	m.mu.Unlock()

	if proteins == "" || hmm == "" {
		return errors.New("proteins and HMM are required for TF identification")
	}
	out := filepath.Join(dir, "tf_genes.tsv")
	args := []string{
		"identify-tfs",
		"--proteins", proteins,
		"--hmm", hmm,
		"--out", out,
	}
	if familyMap != "" {
		args = append(args, "--family-map", familyMap)
	}
	cmd := exec.CommandContext(ctx, m.bin, args...)
	subproc.HideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("identify TFs: %s", msg)
	}
	m.mu.Lock()
	js.tfGenesPath = out
	js.Message = "TF catalogue prepared"
	js.UpdatedAt = time.Now()
	m.mu.Unlock()
	return nil
}

func (m *Manager) buildArgs(js *jobState, mode string) []string {
	if js.referencePackDir != "" {
		sequenceFlag := "--proteins"
		if js.sequenceIsCDS {
			sequenceFlag = "--cds"
		}
		args := []string{
			"pipeline",
			"--reference-pack", js.referencePackDir,
			"--genome", js.genomePath,
			"--gff", js.gffPath,
			sequenceFlag, js.proteinsPath,
			"--out-dir", js.pipelineDir,
			"--promoter-len", strconv.Itoa(js.Params.PromoterLen),
			"--pvalue", strconv.FormatFloat(js.Params.PValue, 'g', -1, 64),
		}
		if js.Params.BackgroundMode == "empirical" {
			args = append(args, "--background-mode", "empirical")
		} else {
			args = append(args, "--background", fmt.Sprintf("%g,%g,%g,%g", js.Params.Background[0], js.Params.Background[1], js.Params.Background[2], js.Params.Background[3]))
		}
		if js.Params.ReuseExactPWM {
			args = append(args, "--reuse-exact-pwm")
		}
		if js.Params.Phase3Artifacts {
			args = append(args, "--phase3-artifacts")
		}
		if js.Params.UseScanCache {
			args = append(args, "--scan-cache", filepath.Join(js.dir, "scan-cache"))
		}
		if js.annotationPath != "" {
			args = append(args, "--annotation", js.annotationPath)
		}
		if js.Params.Downstream > 0 {
			args = append(args, "--downstream", strconv.Itoa(js.Params.Downstream))
		}
		if js.Params.Threads > 0 {
			args = append(args, "--threads", strconv.Itoa(js.Params.Threads))
		}
		if js.Params.MappingMode != "" {
			args = append(args, "--mapping-mode", js.Params.MappingMode)
		}
		if js.Params.SitesOnly {
			args = append(args, "--sites-only")
		}
		if js.Params.IncludeSingleDomainFallback {
			args = append(args, "--include-single-domain-fallback")
		}
		if value := m.toolPath("hmmsearch"); value != "" {
			args = append(args, "--hmmsearch", value)
		}
		if value := m.toolPath("blastp"); value != "" {
			args = append(args, "--blastp", value)
		}
		return args
	}
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
		"--report-json", filepath.Join(js.dir, "report.json"),
	)
	if js.Params.BackgroundMode == "empirical" {
		args = append(args, "--background-mode", "empirical")
	} else {
		args = append(args, "--background", fmt.Sprintf("%g,%g,%g,%g", js.Params.Background[0], js.Params.Background[1], js.Params.Background[2], js.Params.Background[3]))
	}
	if js.Params.ReuseExactPWM {
		args = append(args, "--reuse-exact-pwm")
	}
	if js.Params.UseScanCache {
		args = append(args, "--scan-cache", filepath.Join(js.dir, "scan-cache"))
	}
	if js.Params.Phase3Artifacts {
		args = append(args,
			"--sites-out", js.sitesPath,
			"--motif-testability-out", js.testabilityPath,
			"--promoter-summary-out", js.promoterSummaryPath,
			"--scan-manifest", js.scanManifestPath,
		)
		if js.Params.SitesOnly {
			args = append(args, "--sites-only")
		}
	}
	if js.tfMapPath != "" {
		args = append(args, "--tf-map", js.tfMapPath)
	}
	if js.tfGenesPath != "" {
		args = append(args, "--tf-genes", js.tfGenesPath)
	}
	if js.annotationPath != "" {
		args = append(args, "--annotation", js.annotationPath)
	}
	if js.Params.Threads > 0 {
		args = append(args, "--threads", strconv.Itoa(js.Params.Threads))
	}
	if js.Params.IncludeSingleDomainFallback {
		args = append(args, "--include-single-domain-fallback")
	}
	if mode == "run" {
		args = append(args,
			"--out", js.evidencePath,
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
	report, reportErr := readReport(js.reportPath)
	if reportErr == nil {
		sanitizeReport(report, js.dir)
		if b, err := json.MarshalIndent(report, "", "  "); err == nil {
			_ = os.WriteFile(js.reportPath, b, 0o644)
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
	promoters, err := readPromoters(js.promotersPath)
	if err != nil {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "failed to read promoters"
		js.Error = err.Error()
		return
	}
	var data *dataset
	if js.Params.SitesOnly {
		mappings := map[string][]TFMappingRow{}
		if js.phase4 != nil {
			mappings = js.phase4.Mappings
		} else if rows, err := readTFMappingRows(js.tfMapPath); err == nil {
			mappings = rows
		}
		data, err = buildSiteDataset(
			js.sitesPath, js.motifs, js.annotations, len(promoters), js.annotationSource,
			mappings, js.Params.IncludeSingleDomainFallback,
		)
	} else {
		data, err = buildDataset(
			js.evidencePath,
			js.motifs,
			js.annotations,
			len(promoters),
			js.annotationSource,
			reportContextInt(report, "SCAN_SUMMARY", "motif_hits"),
		)
	}
	if err != nil {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "failed to index evidence"
		js.Error = err.Error()
		return
	}
	data.summary.Threads = reportContextInt(report, "SCAN_SUMMARY", "threads")
	js.enrichSummaryNames(data)
	js.loadPhase4Metadata(report)
	if err := js.addTFCatalogue(data.summary, data); err != nil {
		js.Status, js.Stage, js.Message, js.Error = "failed", "failed", "failed to read TF catalogue", err.Error()
		return
	}
	js.Summary = data.summary
	js.enrichPhase4Summary(report)
	donors, err := loadDonors(js.tracePath, js.tfMapPath)
	if err != nil {
		js.Status = "failed"
		js.Stage = "failed"
		js.Message = "failed to read trace donor table"
		js.Error = err.Error()
		return
	}
	js.edges = nil
	js.data = data
	js.promoters = promoters
	js.donors = donors
	js.Summary = data.summary
	js.Status = "succeeded"
	js.Stage = "succeeded"
	if js.Params.SitesOnly {
		js.Message = fmt.Sprintf("%d candidate sites (site-only)", data.total)
	} else {
		js.Message = fmt.Sprintf("%d evidence rows", data.total)
	}
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
		if p, err := readProgress(js.progressPath); err == nil {
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
		return QueryResult{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return QueryResult{}, ErrNotCompleted
	}
	return js.query(opts), nil
}

// PromoterView returns one target's promoter and grouped binding sites.
func (m *Manager) PromoterView(id, target string) (*PromoterView, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return nil, ErrNotCompleted
	}
	return js.promoterView(target)
}

// SiteEvidence returns the original evidence rows for one exact site.
func (m *Manager) SiteEvidence(id string, opts SiteEvidenceOptions) (SiteEvidenceResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return SiteEvidenceResult{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return SiteEvidenceResult{}, ErrNotCompleted
	}
	return js.siteEvidence(opts)
}

// ExportPath returns an on-disk artifact and a download filename.
func (m *Manager) ExportPath(id, kind string) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return "", "", fmt.Errorf("%w: job not found", ErrNotFound)
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
		return "", "", fmt.Errorf("%w: unknown export kind %q", ErrUnsupportedFormat, kind)
	}
	path := filepath.Join(js.dir, name)
	switch kind {
	case "evidence":
		if js.evidencePath != "" {
			path = js.evidencePath
		}
	case "report", "diagnostics":
		if js.reportPath != "" {
			path = js.reportPath
		}
	case "promoters":
		if js.promotersPath != "" {
			path = js.promotersPath
		}
	}
	if _, err := os.Stat(path); err != nil {
		return "", "", fmt.Errorf("%w: %s artifact missing", ErrNotFound, kind)
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
	// Report the measured fraction within the current stage, without guessed weights.
	if stage == "done" || stage == "succeeded" {
		return 1
	}
	return local
}
