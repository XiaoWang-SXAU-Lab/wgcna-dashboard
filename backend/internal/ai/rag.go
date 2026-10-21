package ai

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode"
)

// This is a lightweight, dependency-free vector embedding: a hashed bag-of-words
// normalized to unit length. It enables vector-similarity retrieval without a
// neural model or ONNX runtime. The Embed signature is stable, so it can be
// swapped for a real ONNX embedding later without touching the retrieval layer.

const defaultDim = 256

func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// Embed tokenizes text into a normalized fixed-dim vector.
func Embed(text string) []float32 {
	return EmbedDim(text, defaultDim)
}

// EmbedDim is Embed with a configurable dimension.
func EmbedDim(text string, dim int) []float32 {
	vec := make([]float32, dim)
	var norm float64
	for _, tok := range tokenize(text) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(tok))
		i := int(h.Sum32() % uint32(dim))
		vec[i]++
		norm += 1
	}
	if norm > 0 {
		inv := float32(1.0 / math.Sqrt(norm))
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec
}

func cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na)*float64(nb)))
}

// Chunk is a retrievable piece of knowledge.
type Chunk struct {
	Source string
	Text   string
	Vec    []float32
}

// KB is a local vector knowledge base.
type KB struct {
	chunks []Chunk
}

// NewKB chunks the documents and embeds each chunk.
func NewKB(docs map[string]string, chunkSize int) *KB {
	kb := &KB{}
	for source, text := range docs {
		for _, part := range splitChunks(text, chunkSize) {
			kb.chunks = append(kb.chunks, Chunk{
				Source: source,
				Text:   part,
				Vec:    Embed(part),
			})
		}
	}
	return kb
}

// Search returns the top-k most relevant chunks for a query.
func (kb *KB) Search(query string, k int) []Chunk {
	q := Embed(query)
	type scored struct {
		chunk Chunk
		score float32
	}
	var results []scored
	for _, c := range kb.chunks {
		results = append(results, scored{c, cosine(q, c.Vec)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if k > len(results) {
		k = len(results)
	}
	out := make([]Chunk, k)
	for i := 0; i < k; i++ {
		out[i] = results[i].chunk
	}
	return out
}

func splitChunks(text string, size int) []string {
	if size <= 0 {
		size = 800
	}
	var chunks []string
	words := strings.Fields(text)
	for len(words) > 0 {
		n := size
		if n > len(words) {
			n = len(words)
		}
		chunks = append(chunks, strings.Join(words[:n], " "))
		words = words[n:]
	}
	if len(chunks) == 0 {
		chunks = append(chunks, text)
	}
	return chunks
}
