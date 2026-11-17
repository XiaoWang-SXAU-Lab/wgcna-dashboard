package regutrace

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// IDMapPreview is the local validation result for a user-supplied
// original_id -> gene_id mapping table. The raw file never leaves the app.
type IDMapPreview struct {
	Rows              int      `json:"rows"`
	UniqueSources     int      `json:"unique_sources"`
	SourceFound       int      `json:"source_found"`
	TargetFound       int      `json:"target_found"`
	Conflicts         int      `json:"conflicts"`
	DuplicateRows     int      `json:"duplicate_rows"`
	MappedTargets     int      `json:"mapped_targets"`
	TotalGenes        int      `json:"total_genes"`
	Coverage          float64  `json:"coverage"`
	AnnotationEntries int      `json:"annotation_entries"`
	AppliedSources    int      `json:"applied_sources,omitempty"`
	Overwritten       int      `json:"overwritten,omitempty"`
	RepairedRows      int      `json:"repaired_rows,omitempty"`
	UnmatchedSources  []string `json:"unmatched_sources,omitempty"`
	MissingTargets    []string `json:"missing_targets,omitempty"`
	ConflictExamples  []string `json:"conflict_examples,omitempty"`
}

type idMapRow struct {
	Source string
	Target string
}

type idMapParse struct {
	Rows          []idMapRow
	DuplicateRows int
	Conflicts     []string
}

func normalizeMapHeader(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	value = strings.TrimPrefix(value, "#")
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func findMapColumn(header []string, names ...string) int {
	for i, value := range header {
		for _, name := range names {
			if value == name {
				return i
			}
		}
	}
	return -1
}

func parseIDMap(data []byte) (idMapParse, error) {
	text := strings.TrimPrefix(string(data), "\ufeff")
	delimiter := rune('\t')
	firstLine := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			firstLine = line
			break
		}
	}
	if firstLine != "" && !strings.Contains(firstLine, "\t") && strings.Contains(firstLine, ",") {
		delimiter = ','
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return idMapParse{}, fmt.Errorf("parse ID mapping: %w", err)
	}
	headerIdx := -1
	sourceCol, targetCol := -1, -1
	for i, rec := range records {
		if len(rec) < 2 {
			continue
		}
		header := make([]string, len(rec))
		for j, value := range rec {
			header[j] = normalizeMapHeader(value)
		}
		source := findMapColumn(header,
			"original_id", "source_id", "query", "protein_id", "transcript_id",
			"mrna_id", "cds_id", "old_id", "from")
		target := findMapColumn(header, "gene_id", "target_id", "locus_tag", "gene", "to")
		if source < 0 && target >= 0 {
			for j := range header {
				if j != target {
					source = j
					break
				}
			}
		}
		if source >= 0 && target >= 0 && source != target {
			headerIdx, sourceCol, targetCol = i, source, target
			break
		}
	}
	if headerIdx < 0 {
		return idMapParse{}, errors.New("ID mapping needs an original/source ID column and a gene_id/target_id column")
	}

	bySource := map[string]string{}
	seen := map[string]bool{}
	out := idMapParse{}
	for _, rec := range records[headerIdx+1:] {
		if len(rec) == 0 {
			continue
		}
		source := strings.TrimSpace(rec[sourceCol])
		if source == "" || strings.HasPrefix(source, "#") {
			continue
		}
		if targetCol >= len(rec) {
			return idMapParse{}, fmt.Errorf("mapping row %q has no target gene ID", source)
		}
		target := strings.TrimSpace(rec[targetCol])
		if target == "" {
			return idMapParse{}, fmt.Errorf("mapping row %q has an empty gene ID", source)
		}
		key := source + "\x00" + target
		if seen[key] {
			out.DuplicateRows++
			continue
		}
		seen[key] = true
		if previous, ok := bySource[source]; ok {
			if previous != target {
				out.Conflicts = append(out.Conflicts, fmt.Sprintf("%s -> %s / %s", source, previous, target))
			}
			continue
		}
		bySource[source] = target
		out.Rows = append(out.Rows, idMapRow{Source: source, Target: target})
	}
	if len(out.Rows) == 0 {
		return idMapParse{}, errors.New("ID mapping has no usable rows")
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		if out.Rows[i].Source != out.Rows[j].Source {
			return out.Rows[i].Source < out.Rows[j].Source
		}
		return out.Rows[i].Target < out.Rows[j].Target
	})
	sort.Strings(out.Conflicts)
	return out, nil
}

func cleanGFFID(value string) string {
	value = strings.TrimSpace(strings.Trim(value, `"`))
	for _, prefix := range []string{"gene:", "transcript:", "mRNA:", "rna:"} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return value
}

func parseGFFAttrs(value string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(value, ";") {
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = cleanGFFID(val)
		if key != "" && val != "" {
			out[key] = val
		}
	}
	return out
}

func openMaybeGzip(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	buffered := bufio.NewReader(file)
	magic, _ := buffered.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		return &gzipReadCloser{Reader: gz, close: func() error {
			_ = gz.Close()
			return file.Close()
		}}, nil
	}
	return &gzipReadCloser{Reader: buffered, close: file.Close}, nil
}

type gzipReadCloser struct {
	io.Reader
	close func() error
}

func (g *gzipReadCloser) Close() error { return g.close() }

func loadGFFGenes(path string) (map[string]bool, error) {
	reader, err := openMaybeGzip(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	genes := map[string]bool{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "##FASTA") {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 9 || fields[2] != "gene" {
			continue
		}
		attrs := parseGFFAttrs(fields[8])
		id := attrs["ID"]
		if id == "" {
			id = attrs["Name"]
		}
		if id != "" {
			genes[id] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(genes) == 0 {
		return nil, errors.New("GFF has no gene IDs")
	}
	return genes, nil
}

func previewIDMapping(rows idMapParse, annotations map[string]AnnotationEntry, genes map[string]bool) IDMapPreview {
	preview := IDMapPreview{
		Rows:          len(rows.Rows),
		UniqueSources: len(rows.Rows),
		Conflicts:     len(rows.Conflicts),
		DuplicateRows: rows.DuplicateRows,
		TotalGenes:    len(genes),
	}
	targets := map[string]bool{}
	unmatchedSources := map[string]bool{}
	missingTargets := map[string]bool{}
	for _, row := range rows.Rows {
		_, sourceOK := annotations[row.Source]
		if sourceOK {
			preview.SourceFound++
		} else if len(unmatchedSources) < 5 {
			unmatchedSources[row.Source] = true
		}
		targetOK := genes[row.Target]
		if targetOK {
			preview.TargetFound++
		} else if len(missingTargets) < 5 {
			missingTargets[row.Target] = true
		}
		if sourceOK && targetOK {
			targets[row.Target] = true
		}
	}
	preview.MappedTargets = len(targets)
	preview.AnnotationEntries = len(targets)
	if preview.TotalGenes > 0 {
		preview.Coverage = float64(preview.MappedTargets) * 100 / float64(preview.TotalGenes)
	}
	for value := range unmatchedSources {
		preview.UnmatchedSources = append(preview.UnmatchedSources, value)
	}
	for value := range missingTargets {
		preview.MissingTargets = append(preview.MissingTargets, value)
	}
	sort.Strings(preview.UnmatchedSources)
	sort.Strings(preview.MissingTargets)
	preview.ConflictExamples = append([]string(nil), rows.Conflicts...)
	if len(preview.ConflictExamples) > 5 {
		preview.ConflictExamples = preview.ConflictExamples[:5]
	}
	return preview
}

// PreviewIDMapping validates a mapping file against the job's annotation keys
// and GFF gene IDs without modifying anything.
func (m *Manager) PreviewIDMapping(id string, data []byte) (IDMapPreview, error) {
	rows, err := parseIDMap(data)
	if err != nil {
		return IDMapPreview{}, err
	}
	m.mu.RLock()
	js, ok := m.jobs[id]
	if !ok {
		m.mu.RUnlock()
		return IDMapPreview{}, errors.New("job not found")
	}
	gffPath := js.gffPath
	annotations := make(map[string]AnnotationEntry, len(js.annotations))
	for key, entry := range js.annotations {
		annotations[key] = entry
	}
	m.mu.RUnlock()
	if gffPath == "" {
		return IDMapPreview{}, errors.New("job has no GFF; ID mapping preview needs a GFF with gene features")
	}
	if len(annotations) == 0 {
		return IDMapPreview{}, errors.New("job has no annotation table to remap")
	}
	genes, err := loadGFFGenes(gffPath)
	if err != nil {
		return IDMapPreview{}, err
	}
	return previewIDMapping(rows, annotations, genes), nil
}

// ApplyIDMapping remaps annotation entries to GFF gene-level keys, writes a
// repaired annotation.tsv copy, and leaves the original annotation file intact.
func (m *Manager) ApplyIDMapping(id string, data []byte) (IDMapPreview, error) {
	rows, err := parseIDMap(data)
	if err != nil {
		return IDMapPreview{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	js, ok := m.jobs[id]
	if !ok {
		return IDMapPreview{}, errors.New("job not found")
	}
	if js.gffPath == "" {
		return IDMapPreview{}, errors.New("job has no GFF; ID mapping apply needs a GFF with gene features")
	}
	if len(js.annotations) == 0 {
		return IDMapPreview{}, errors.New("job has no annotation table to remap")
	}
	genes, err := loadGFFGenes(js.gffPath)
	if err != nil {
		return IDMapPreview{}, err
	}
	preview := previewIDMapping(rows, js.annotations, genes)
	if preview.Conflicts > 0 {
		return preview, fmt.Errorf("ID mapping has %d conflicting source(s); fix the mapping before applying", preview.Conflicts)
	}
	if len(preview.MissingTargets) > 0 {
		return preview, fmt.Errorf("ID mapping has target gene IDs absent from the GFF: %s", strings.Join(preview.MissingTargets, ", "))
	}
	if preview.SourceFound == 0 {
		return preview, errors.New("none of the mapping source IDs occur in this job's annotation table")
	}

	repaired := map[string]AnnotationEntry{}
	for key, entry := range js.annotations {
		if genes[key] {
			repaired[key] = entry
		}
	}
	applied, overwritten := 0, 0
	for _, row := range rows.Rows {
		entry, ok := js.annotations[row.Source]
		if !ok || !genes[row.Target] {
			continue
		}
		if _, exists := repaired[row.Target]; exists {
			overwritten++
		}
		repaired[row.Target] = entry
		applied++
	}
	if applied == 0 {
		return preview, errors.New("no mapping rows could be applied")
	}

	if js.annotationPath != "" {
		original := filepath.Join(js.dir, "annotation.original.tsv")
		if _, err := os.Stat(original); errors.Is(err, os.ErrNotExist) {
			if old, readErr := os.ReadFile(js.annotationPath); readErr == nil {
				_ = os.WriteFile(original, old, 0o644)
			}
		}
	}
	annotationPath := filepath.Join(js.dir, "annotation.tsv")
	if err := writeRepairedAnnotation(annotationPath, repaired); err != nil {
		return preview, err
	}
	mappingPath := filepath.Join(js.dir, "id_mapping.tsv")
	if err := writeIDMap(mappingPath, rows.Rows); err != nil {
		return preview, err
	}

	js.annotationPath = annotationPath
	js.annotationSource = "mapping"
	js.annotations = withVersionAliases(repaired)
	preview.AppliedSources = applied
	preview.Overwritten = overwritten
	preview.RepairedRows = len(repaired)
	preview.AnnotationEntries = len(repaired)
	return preview, nil
}

func writeRepairedAnnotation(path string, annotations map[string]AnnotationEntry) error {
	genes := make([]string, 0, len(annotations))
	for gene := range annotations {
		genes = append(genes, gene)
	}
	sort.Strings(genes)
	var out strings.Builder
	out.WriteString("gene_id\tsymbol\tdescription\n")
	for _, gene := range genes {
		entry := annotations[gene]
		symbol := entry.Symbol
		if symbol == "" {
			symbol = gene
		}
		out.WriteString(gene + "\t" + symbol + "\t" + entry.Description + "\n")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0o644); err != nil {
		return fmt.Errorf("write repaired annotation: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("publish repaired annotation: %w", err)
	}
	return nil
}

func writeIDMap(path string, rows []idMapRow) error {
	var out strings.Builder
	out.WriteString("original_id\tgene_id\n")
	for _, row := range rows {
		out.WriteString(row.Source + "\t" + row.Target + "\n")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0o644); err != nil {
		return fmt.Errorf("write normalized ID mapping: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("publish normalized ID mapping: %w", err)
	}
	return nil
}
