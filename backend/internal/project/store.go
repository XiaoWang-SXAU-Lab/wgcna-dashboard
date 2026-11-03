// Package project provides versioned, relocatable analysis projects. It is
// independent of the legacy job registry so old jobs are never silently trusted.
package project

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = 1

var ErrConflict = errors.New("project changed; reload before saving")

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type QuestionCard struct {
	Question    string `json:"question"`
	Hypothesis  string `json:"hypothesis,omitempty"`
	Organism    string `json:"organism,omitempty"`
	Design      string `json:"design,omitempty"`
	Limitations string `json:"limitations,omitempty"`
}

// Execution parameters are immutable provenance; Draft is the editable state.
type Step struct {
	ExecutionID string            `json:"execution_id"`
	Status      string            `json:"status"`
	Params      json.RawMessage   `json:"params"`
	Engine      string            `json:"engine"`
	Inputs      map[string]string `json:"inputs"`
	Upstream    map[string]string `json:"upstream"`
	Outputs     []Artifact        `json:"outputs"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// Progress mirrors the worker's progress.json written into the staging dir.
type Progress struct {
	Step     string  `json:"step"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

type State struct {
	SchemaVersion int                        `json:"schema_version"`
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Revision      uint64                     `json:"revision"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	Question      QuestionCard               `json:"question_card"`
	Inputs        map[string]Artifact        `json:"inputs"`
	Draft         map[string]json.RawMessage `json:"draft_params"`
	UI            json.RawMessage            `json:"ui_state"`
	Steps         map[string]Step            `json:"steps"`
}

type Project struct {
	mu       sync.Mutex
	root     string
	lock     *os.File
	state    State
	closed   bool
	active   bool     // Actual live worker, independent of a failed manifest write.
	progress Progress // Latest worker progress; zero before a run.
}

func identifier() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// Create requires a new directory and never overwrites an existing project.
func Create(root, name string) (*Project, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("project name required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.Mkdir(abs, 0o755); err != nil {
		return nil, err
	}
	p, err := acquire(abs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p.state = State{SchemaVersion: SchemaVersion, ID: identifier(), Name: name, Revision: 1, CreatedAt: now, UpdatedAt: now,
		Inputs: map[string]Artifact{}, Draft: map[string]json.RawMessage{}, UI: json.RawMessage(`{}`), Steps: map[string]Step{}}
	if err = p.write(p.state); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func acquire(root string) (*Project, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	root = resolved
	// OS locks release on process exit; a leftover lock file is harmless. They
	// protect local cooperating processes, not distributed sync-client replicas.
	lockPath := filepath.Join(root, ".project.lock")
	if info, e := os.Lstat(lockPath); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("symlink lock rejected")
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("project already open or cannot lock: %w", err)
	}
	return &Project{root: root, lock: f}, nil
}

func Open(root string) (*Project, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	p, err := acquire(abs)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Project, error) { _ = p.lock.Close(); p.closed = true; return nil, err }
	b, err := os.ReadFile(filepath.Join(abs, "project.json"))
	if err != nil {
		return fail(err)
	}
	if err = json.Unmarshal(b, &p.state); err != nil {
		return fail(fmt.Errorf("invalid project.json (backup may be available): %w", err))
	}
	if err = validate(p.state); err != nil {
		return fail(err)
	}
	// Only after acquiring ownership can previous in-progress work be interrupted.
	// Workers must run in isolated local staging, never directly in this project.
	changed := false
	for name, step := range p.state.Steps {
		if step.Status == "running" {
			step.Status = "interrupted"
			step.Error = "previous owner exited before committing results"
			p.state.Steps[name] = step
			changed = true
		}
	}
	if changed {
		p.state.Revision++
		p.state.UpdatedAt = time.Now().UTC()
		if err = p.write(p.state); err != nil {
			return fail(err)
		}
	}
	return p, nil
}

func validate(s State) error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported project schema %d; expected %d", s.SchemaVersion, SchemaVersion)
	}
	if s.ID == "" || s.Name == "" || s.Revision == 0 || s.Inputs == nil || s.Draft == nil || s.Steps == nil || !json.Valid(s.UI) {
		return errors.New("incomplete project manifest")
	}
	for _, a := range s.Inputs {
		if err := validArtifact(a); err != nil {
			return err
		}
	}
	for _, step := range s.Steps {
		if !json.Valid(step.Params) {
			return errors.New("invalid execution parameters")
		}
		for _, a := range step.Outputs {
			if err := validArtifact(a); err != nil {
				return err
			}
		}
	}
	for _, raw := range s.Draft {
		if !json.Valid(raw) {
			return errors.New("invalid draft parameters")
		}
	}
	return nil
}

func validArtifact(a Artifact) error {
	// Use portable slash paths, rejecting Windows paths even when opened on Mac.
	if a.Path == "" || strings.ContainsAny(a.Path, `\:`) || strings.HasPrefix(a.Path, "/") {
		return errors.New("invalid project-relative artifact path")
	}
	for _, part := range strings.Split(a.Path, "/") {
		if part == ".." || part == "." || part == "" {
			return errors.New("invalid artifact path component")
		}
	}
	hash, err := hex.DecodeString(a.SHA256)
	if err != nil || len(hash) != sha256.Size {
		return errors.New("invalid artifact checksum")
	}
	return nil
}

func clone(s State) State {
	b, _ := json.Marshal(s)
	var out State
	_ = json.Unmarshal(b, &out)
	return out
}

func (p *Project) Snapshot() State { p.mu.Lock(); defer p.mu.Unlock(); return clone(p.state) }

// Save uses optimistic concurrency so a late autosave cannot overwrite newer
// execution results. Identity and input/step records are not editable by UI.
func (p *Project) Save(revision uint64, draft map[string]json.RawMessage, ui json.RawMessage, card QuestionCard) (State, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return State{}, errors.New("project closed")
	}
	if revision != p.state.Revision {
		return State{}, ErrConflict
	}
	s := clone(p.state)
	s.Draft = draft
	s.UI = ui
	s.Question = card
	if err := p.commit(s); err != nil {
		return State{}, err
	}
	return clone(p.state), nil
}

func (p *Project) commit(s State) error {
	s.Revision++
	s.UpdatedAt = time.Now().UTC()
	if err := validate(s); err != nil {
		return err
	}
	if err := p.write(s); err != nil {
		return err
	}
	p.state = clone(s)
	return nil
}

func (p *Project) write(s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(p.root, "project.json")
	if previous, e := os.ReadFile(path); e == nil {
		var old State
		if json.Unmarshal(previous, &old) != nil || validate(old) != nil {
			return errors.New("refusing to replace invalid manifest; recover backup explicitly")
		}
		if err = atomicWrite(path+".bak", previous); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	return atomicWrite(path, data)
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".project-write-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Windows replacement uses MoveFileEx rather than remove-then-rename.
	return replaceFile(tmp, path)
}

func (p *Project) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if p.active {
		p.closed = false
		return errors.New("cancel and finish the running step before closing project")
	}
	return p.lock.Close()
}

func digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Resolve rejects symlink components: shared manifests may not reference
// files outside their project through either path traversal or symlinks.
func (p *Project) Resolve(a Artifact) (string, error) {
	if err := validArtifact(a); err != nil {
		return "", err
	}
	path := p.root
	for _, part := range strings.Split(a.Path, "/") {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlink artifact rejected")
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("artifact must be regular file")
	}
	got, err := digest(path)
	if err != nil {
		return "", err
	}
	if got != a.SHA256 {
		return "", fmt.Errorf("artifact changed: %s", a.Path)
	}
	return path, nil
}

// Import copies a data file into an immutable, project-relative location.
func (p *Project) Import(role string, source io.Reader) (State, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return State{}, errors.New("project closed")
	}
	switch role {
	case "expr", "trait", "sample_info", "annotation", "gene2go", "gene2kegg":
	default:
		return State{}, errors.New("unsupported input role")
	}
	dir, err := p.safeDirectory("inputs/" + identifier())
	if err != nil {
		return State{}, err
	}
	name := filepath.Join(dir, role+".tsv")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return State{}, err
	}
	_, err = io.Copy(f, source)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return State{}, err
	}
	if closeErr != nil {
		return State{}, closeErr
	}
	hash, err := digest(name)
	if err != nil {
		return State{}, err
	}
	rel, _ := filepath.Rel(p.root, name)
	s := clone(p.state)
	s.Inputs[role] = Artifact{Path: filepath.ToSlash(rel), SHA256: hash}
	if err = p.commit(s); err != nil {
		return State{}, err
	}
	return clone(p.state), nil
}

// safeDirectory creates only checked components below a canonical project root.
func (p *Project) safeDirectory(relative string) (string, error) {
	if err := validArtifact(Artifact{Path: relative, SHA256: strings.Repeat("0", 64)}); err != nil {
		return "", err
	}
	path := p.root
	for _, part := range strings.Split(relative, "/") {
		path = filepath.Join(path, part)
		if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("project directory must not be a symlink or file")
		}
	}
	return path, nil
}
