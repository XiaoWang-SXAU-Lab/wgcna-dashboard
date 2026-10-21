package ai

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := encrypt("sk-test-abc123", key)
	if err != nil {
		t.Fatal(err)
	}
	if enc == "sk-test-abc123" {
		t.Fatal("ciphertext must not equal plaintext")
	}
	dec, err := decrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "sk-test-abc123" {
		t.Fatalf("round-trip mismatch: got %q", dec)
	}

	// wrong key must fail
	bad := make([]byte, 32)
	if _, err := decrypt(enc, bad); err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestEmbedCosine(t *testing.T) {
	a := Embed("soft threshold beta scale free network")
	b := Embed("soft threshold beta scale free network")
	if got := cosine(a, b); got < 0.999 {
		t.Fatalf("identical vectors cosine = %v, want ~1", got)
	}
	c := Embed("completely different words here")
	if got := cosine(a, c); got > 0.5 {
		t.Fatalf("unrelated vectors cosine = %v, want low", got)
	}
}

func TestKBRetrieval(t *testing.T) {
	kb := NewKB(KnowledgeDocs(), 800)
	if len(kb.chunks) == 0 {
		t.Fatal("empty knowledge base")
	}
	// query about soft threshold should surface params/principles docs
	res := kb.Search("软阈值 beta 怎么选择", 3)
	if len(res) == 0 {
		t.Fatal("no retrieval results")
	}
	t.Logf("query '软阈值 beta 怎么选择' -> top sources: %s, %s, %s",
		res[0].Source, res[1].Source, res[2].Source)

	// query about errors should surface troubleshooting
	res2 := kb.Search("read trait.tsv 报错怎么办", 2)
	if len(res2) == 0 {
		t.Fatal("no results for error query")
	}
	t.Logf("query 'read trait.tsv 报错怎么办' -> top source: %s", res2[0].Source)
}
