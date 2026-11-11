package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/demo"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/evidence"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/project"
)

type projectSession struct {
	mu        sync.Mutex
	p         *project.Project
	validity  map[string]string
	busy      bool
	cancel    context.CancelFunc
	lastError string
}
type recentProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}
type ProjectHandler struct {
	mu                            sync.Mutex
	sessions                      map[string]*projectSession
	recent                        []recentProject
	worker, engine, home, staging string
	closing                       bool
	wg                            sync.WaitGroup
}

func NewProjectHandler(worker, home, staging string) (*ProjectHandler, error) {
	engine, err := project.EngineFingerprint(worker)
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{home, staging} {
		if err = os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	h := &ProjectHandler{sessions: map[string]*projectSession{}, worker: worker, engine: engine, home: home, staging: staging, recent: []recentProject{}}
	if b, e := os.ReadFile(filepath.Join(home, "recent-projects.json")); e == nil {
		_ = json.Unmarshal(b, &h.recent)
	}
	return h, nil
}

func (h *ProjectHandler) Register(mux *http.ServeMux) {
	routes := map[string]http.HandlerFunc{
		"POST /api/projects": h.create, "POST /api/projects/open": h.open,
		"GET /api/projects/recent": h.recents, "GET /api/projects/{id}": h.get,
		"PUT /api/projects/{id}/draft": h.save, "POST /api/projects/{id}/inputs/{role}": h.importInput,
		"POST /api/projects/{id}/steps/{step}/run": h.run, "GET /api/projects/{id}/results/{step}": h.result,
		"POST /api/projects/{id}/cancel": h.cancelRun, "POST /api/projects/{id}/close": h.closeProject,
		"POST /api/projects/{id}/demo":                     h.loadDemo,
		"GET /api/projects/{id}/sample-info":               h.sampleInfo,
		"GET /api/projects/{id}/plots/{kind}":              h.plotProject,
		"GET /api/projects/{id}/annotation":                h.projectAnnotation,
		"GET /api/projects/{id}/enrich":                    h.projectEnrich,
		"GET /api/projects/{id}/expression":                h.projectExpression,
		"POST /api/projects/{id}/tf-target/upstream":       h.tfTargetUpstream,
		"GET /api/projects/{id}/outputs/{step}/{basename}": h.downloadOutput,
		"GET /api/projects/{id}/report/hub-priority":       h.candidateReport,
		"GET /api/projects/{id}/evidence/hub-priority":     h.hubPriorityEvidence,
	}
	for pattern, fn := range routes {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			// Desktop filesystem endpoints accept only same-origin browser requests.
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "cross-site request rejected", 403)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					http.Error(w, "origin rejected", 403)
					return
				}
			}
			w.Header().Set("Cache-Control", "no-store")
			fn(w, r)
		})
	}
}

func decodeProject(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		http.Error(w, err.Error(), 400)
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		http.Error(w, "one JSON object required", 400)
		return false
	}
	return true
}

func (h *ProjectHandler) session(w http.ResponseWriter, r *http.Request) *projectSession {
	h.mu.Lock()
	s := h.sessions[r.PathValue("id")]
	closed := h.closing
	h.mu.Unlock()
	if closed {
		http.Error(w, "application shutting down", 503)
		return nil
	}
	if s == nil {
		http.Error(w, "open this project first", 404)
	}
	return s
}

func (h *ProjectHandler) reply(w http.ResponseWriter, s *projectSession, status int) {
	writeJSON(w, status, map[string]any{"state": s.p.Snapshot(), "validity": s.validity, "busy": s.busy, "error": s.lastError, "path": s.p.Root(), "progress": s.p.Progress()})
}

func (h *ProjectHandler) attach(p *project.Project) (*projectSession, error) {
	id := p.Snapshot().ID
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		_ = p.Close()
		return nil, errors.New("application shutting down")
	}
	if _, exists := h.sessions[id]; exists {
		_ = p.Close()
		return nil, errors.New("a project with this identity is already open; close it before opening a copy")
	}
	s := &projectSession{p: p, validity: p.Validity(h.engine)}
	h.sessions[id] = s
	item := recentProject{ID: id, Name: p.Snapshot().Name, Path: p.Root()}
	recent := []recentProject{item}
	for _, old := range h.recent {
		if old.Path != item.Path {
			recent = append(recent, old)
		}
		if len(recent) >= 12 {
			break
		}
	}
	b, _ := json.MarshalIndent(recent, "", "  ")
	f, err := os.CreateTemp(h.home, ".recents-*")
	if err == nil {
		tmp := f.Name()
		defer os.Remove(tmp)
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmp, filepath.Join(h.home, "recent-projects.json"))
		}
	}
	if err != nil {
		delete(h.sessions, id)
		_ = p.Close()
		return nil, fmt.Errorf("save recent projects: %w", err)
	}
	h.recent = recent
	return s, nil
}

func (h *ProjectHandler) create(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if !decodeProject(w, r, &q) {
		return
	}
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" || q.Name == "." || q.Name == ".." || strings.ContainsAny(q.Name, `/\:`) || !filepath.IsAbs(q.Parent) {
		http.Error(w, "choose an absolute parent directory and a valid project name", 400)
		return
	}
	p, err := project.Create(filepath.Join(q.Parent, q.Name), q.Name)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s, err := h.attach(p)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h.reply(w, s, 201)
}

func (h *ProjectHandler) open(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Path string `json:"path"`
	}
	if !decodeProject(w, r, &q) {
		return
	}
	if !filepath.IsAbs(q.Path) {
		http.Error(w, "absolute project directory required", 400)
		return
	}
	root, err := filepath.EvalSymlinks(q.Path)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	root, _ = filepath.Abs(root)
	h.mu.Lock()
	var existing *projectSession
	for _, s := range h.sessions {
		if s.p.Root() == root || (strings.EqualFold(s.p.Root(), root) && os.PathSeparator == '\\') {
			existing = s
			break
		}
	}
	h.mu.Unlock()
	if existing != nil {
		existing.mu.Lock()
		defer existing.mu.Unlock()
		h.reply(w, existing, 200)
		return
	}
	p, err := project.Open(root)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	s, err := h.attach(p)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h.reply(w, s, 200)
}

func (h *ProjectHandler) recents(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	writeJSON(w, 200, h.recent)
}
func (h *ProjectHandler) get(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h.reply(w, s, 200)
}

func (h *ProjectHandler) save(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	var q struct {
		Revision uint64                     `json:"revision"`
		Draft    map[string]json.RawMessage `json:"draft_params"`
		UI       json.RawMessage            `json:"ui_state"`
		Question project.QuestionCard       `json:"question_card"`
	}
	if !decodeProject(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		http.Error(w, "wait for the running step before editing", 409)
		return
	}
	_, err := s.p.Save(q.Revision, q.Draft, q.UI, q.Question)
	if err != nil {
		code := 400
		if errors.Is(err, project.ErrConflict) {
			code = 409
		}
		http.Error(w, err.Error(), code)
		return
	}
	s.validity = s.p.Validity(h.engine)
	h.reply(w, s, 200)
}

func (h *ProjectHandler) importInput(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	if r.PathValue("role") == "tf_target" {
		h.importTFTarget(w, r, s)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	data, err := readFile(r, "file")
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		http.Error(w, "wait for the running step before replacing data", 409)
		return
	}
	_, err = s.p.Import(r.PathValue("role"), bytes.NewReader(data))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.validity = s.p.Validity(h.engine)
	h.reply(w, s, 200)
}

func (h *ProjectHandler) loadDemo(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		http.Error(w, "wait for the running step before replacing data", 409)
		return
	}
	expr, err := demo.Expr()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if _, err = s.p.Import("expr", bytes.NewReader(expr)); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if trait, e := demo.Trait(); e == nil {
		if _, err = s.p.Import("trait", bytes.NewReader(trait)); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	if meta, e := demo.Meta(); e == nil {
		for _, role := range []string{"sample_info", "annotation", "gene2go", "gene2kegg"} {
			if data, ok := meta[role]; ok {
				if _, err = s.p.Import(role, bytes.NewReader(data)); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
			}
		}
	}
	s.validity = s.p.Validity(h.engine)
	h.reply(w, s, 200)
}

func (h *ProjectHandler) sampleInfo(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	data, err := s.p.Input("sample_info")
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	infos, err := enrich.ParseSampleInfo(data)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, infos)
}

func (h *ProjectHandler) run(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	step := r.PathValue("step")
	if !project.IsStep(step) {
		http.Error(w, "unknown step", 400)
		return
	}
	var q struct {
		Threads  int    `json:"threads"`
		Revision uint64 `json:"revision"`
	}
	if !decodeProject(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy || q.Revision != s.p.Snapshot().Revision {
		http.Error(w, "project busy or changed; reload before running", 409)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.busy = true
	s.lastError = ""
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		cancel()
		s.busy = false
		http.Error(w, "application shutting down", 503)
		return
	}
	h.wg.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.wg.Done()
		defer cancel()
		err := s.p.Run(ctx, step, h.worker, h.staging, q.Threads)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.busy = false
		s.cancel = nil
		if err != nil {
			s.lastError = err.Error()
		}
		s.validity = s.p.Validity(h.engine)
	}()
	h.reply(w, s, 202)
}

func (h *ProjectHandler) result(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.p.Result(r.PathValue("step"))
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// downloadOutput serves a read-only committed output file by exact basename.
// Only a precise whitelist is allowed; arbitrary paths are never read.
func (h *ProjectHandler) downloadOutput(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	step := r.PathValue("step")
	basename := r.PathValue("basename")
	if !outputAllowed(step, basename) {
		http.Error(w, "output not allowed", 404)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.p.Output(step, basename)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	switch {
	case strings.HasSuffix(basename, ".tsv"):
		w.Header().Set("Content-Type", "text/tab-separated-values")
	case strings.HasSuffix(basename, ".txt"):
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", basename))
	_, _ = w.Write(data)
}

// outputAllowed is the exact (step, basename) whitelist for downloadable outputs.
func outputAllowed(step, basename string) bool {
	allowed := map[string]map[string]bool{
		"hub_priority": {"step8_hub_priority.tsv": true},
		"filter":       {"step1_filtered.tsv": true, "step1_genes.txt": true},
	}
	return allowed[step][basename]
}

// candidateReport returns the hub_priority report envelope after a SERVER-side
// validity re-check. It never trusts a frontend boolean: a stale/not-run result is
// rejected with a clear reason (409). The envelope carries the report's own
// project/execution/upstream/inputs identity so the frontend can verify provenance.
func (h *ProjectHandler) candidateReport(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	env, err := s.p.CandidateReportEnvelope(h.engine)
	if err != nil {
		// Do not return the underlying filesystem error (it may contain absolute
		// paths or implementation details).
		writeJSON(w, 409, map[string]string{"code": "not_current", "error": candidateReportNotCurrentMessage})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(env)
}

const (
	evidenceNotCurrentMessage        = "candidate-priority evidence is not current or not run"
	candidateReportNotCurrentMessage = "candidate-priority result is not current or not run"
)

// hubPriorityEvidence rebuilds the evidence bundle from the CURRENT atomic
// candidate-report envelope on the server. It never accepts a client-supplied
// envelope or bundle, and returns only the safe ContractError message/code
// (never Diagnostic) on rejection. The deterministic offline suggestion is
// generated by code, not by an online model.
func (h *ProjectHandler) hubPriorityEvidence(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	env, err := s.p.CandidateReportEnvelope(h.engine)
	s.mu.Unlock()
	if err != nil {
		// The underlying envelope error may contain absolute paths or filesystem
		// diagnostics; keep it out of the API response.
		writeJSON(w, http.StatusConflict, map[string]string{"code": "not_current", "error": evidenceNotCurrentMessage})
		return
	}
	resp, status, err := buildHubPriorityEvidence(env, r.URL.Query().Get("full") == "1")
	if err != nil {
		code := evidence.ErrorCode(err)
		if code == "" {
			code = "evidence_rejected"
		}
		writeJSON(w, status, map[string]string{"code": code, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func buildHubPriorityEvidence(env []byte, full bool) (map[string]any, int, error) {
	bundle, err := evidence.BuildBundle(env)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}
	suggestion, err := bundle.OfflineSuggestion()
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}
	raw, err := json.Marshal(suggestion)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	rendered, err := bundle.Evaluate(raw)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}
	resp := map[string]any{
		"project_id":      bundle.ProjectID,
		"execution_id":    bundle.ExecutionID,
		"engine":          bundle.Engine,
		"provenance":      bundle.Provenance,
		"valid":           bundle.Valid,
		"invalid":         bundle.Invalid,
		"gene_count":      bundle.GeneCount,
		"full_coverage":   bundle.FullCoverage,
		"candidate_count": len(bundle.Candidates),
		"fact_count":      len(bundle.Facts),
		"summary":         bundle.Summary,
		"limitations":     bundle.Limitations,
		"offline":         true,
		"suggestion":      suggestion,
		"rendered":        rendered,
	}
	if full {
		resp["facts"] = bundle.Facts
		resp["candidates"] = bundle.Candidates
	}
	return resp, http.StatusOK, nil
}

func (h *ProjectHandler) cancelRun(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	h.reply(w, s, 202)
}
func (h *ProjectHandler) closeProject(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		http.Error(w, "cancel and wait for the step before closing", 409)
		return
	}
	if err := s.p.Close(); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	h.mu.Lock()
	delete(h.sessions, s.p.Snapshot().ID)
	h.mu.Unlock()
	writeJSON(w, 200, map[string]bool{"closed": true})
}

// Shutdown prevents new runs, cancels workers, waits for their final state,
// then releases project locks. Abrupt exit is recovered by project.Open.
func (h *ProjectHandler) Shutdown() {
	h.mu.Lock()
	h.closing = true
	sessions := []*projectSession{}
	for _, s := range h.sessions {
		sessions = append(sessions, s)
	}
	h.mu.Unlock()
	for _, s := range sessions {
		s.mu.Lock()
		if s.cancel != nil {
			s.cancel()
		}
		s.mu.Unlock()
	}
	h.wg.Wait()
	for _, s := range sessions {
		_ = s.p.Close()
	}
}
