package regutrace

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// dataset is a disk-backed evidence index. Large whole-genome scans can emit
// millions of rows; keeping only line offsets by TF and target avoids holding
// the full evidence table in memory while still making queries interactive.
type dataset struct {
	path          string
	schema        *evidenceSchema
	targetOffsets map[string][]int64
	tfOffsets     map[string][]int64
	motifOffsets  map[string][]int64
	tfKeys        map[string][]string
	targetKeys    map[string][]string
	total         int
	summary       *Summary
}

func uniqueStrings(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func buildDataset(path string, motifs map[string]MotifInfo, annotations map[string]AnnotationEntry, promoterCount int, annotationSource string, motifHits int) (*dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("parse evidence TSV header: %w", err)
	}
	schema, err := newEvidenceSchema(header)
	if err != nil {
		return nil, err
	}
	ds := &dataset{
		path:          path,
		schema:        schema,
		targetOffsets: map[string][]int64{},
		tfOffsets:     map[string][]int64{},
		motifOffsets:  map[string][]int64{},
	}
	builder := newSummaryBuilder(false)
	targetSymbols := map[string]string{}
	tfSymbols := map[string]string{}
	last := r.InputOffset()
	row := 1
	for {
		start := last
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse evidence TSV row %d: %w", row+1, err)
		}
		last = r.InputOffset()
		row++
		e, err := schema.edge(rec, row)
		if err != nil {
			return nil, err
		}
		if e.TFID == "" {
			continue
		}
		ds.total++
		ds.targetOffsets[e.TargetID] = append(ds.targetOffsets[e.TargetID], start)
		ds.tfOffsets[e.TFID] = append(ds.tfOffsets[e.TFID], start)
		if e.MotifID != "" {
			ds.motifOffsets[e.MotifID] = append(ds.motifOffsets[e.MotifID], start)
		}
		if _, ok := targetSymbols[e.TargetID]; !ok {
			targetSymbols[e.TargetID] = e.TargetSymbol
		}
		if _, ok := tfSymbols[e.TFID]; !ok {
			tfSymbols[e.TFID] = e.TFSymbol
		}
		builder.add(e, motifs)
	}
	ds.summary = builder.finish(promoterCount, len(motifs), annotationSource, motifHits)
	ds.tfKeys = make(map[string][]string, len(tfSymbols))
	for id, symbol := range tfSymbols {
		ds.tfKeys[id] = uniqueStrings(id, symbol)
	}
	ds.targetKeys = make(map[string][]string, len(targetSymbols))
	for id, symbol := range targetSymbols {
		ann := annotations[id]
		ds.targetKeys[id] = uniqueStrings(id, symbol, ann.Symbol, ann.Description)
	}
	return ds, nil
}

func (ds *dataset) forEachAt(offsets []int64, fn func(Edge) error) error {
	f, err := os.Open(ds.path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, offset := range offsets {
		if offset < 0 {
			continue
		}
		sr := io.NewSectionReader(f, offset, 4<<20)
		r := csv.NewReader(sr)
		r.Comma = '\t'
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read evidence at offset %d: %w", offset, err)
		}
		e, err := ds.schema.edge(rec, 0)
		if err != nil {
			return err
		}
		if e.TFID == "" {
			continue
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func (ds *dataset) forEach(fn func(Edge) error) error {
	f, err := os.Open(ds.path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	if _, err := r.Read(); err != nil {
		return err
	}
	row := 1
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		row++
		e, err := ds.schema.edge(rec, row)
		if err != nil {
			return err
		}
		if e.TFID == "" {
			continue
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

func (ds *dataset) readTargetEdges(targetID string) ([]Edge, error) {
	offsets := ds.targetOffsets[targetID]
	var edges []Edge
	err := ds.forEachAt(offsets, func(e Edge) error {
		edges = append(edges, e)
		return nil
	})
	return edges, err
}

func (ds *dataset) resolveTarget(target string) (string, bool) {
	if _, ok := ds.targetOffsets[target]; ok {
		return target, true
	}
	lower := strings.ToLower(target)
	for id := range ds.targetOffsets {
		if strings.ToLower(id) == lower {
			return id, true
		}
	}
	return "", false
}
