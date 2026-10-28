package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func networkFixture(t *testing.T) *networkIndex {
	t.Helper()
	dir := t.TempDir()
	var nodes, edges strings.Builder
	nodes.WriteString("gene\tmodule\tdegree\n")
	edges.WriteString("source\ttarget\tweight\n")
	for _, module := range []string{"blue", "pink"} {
		for i := 0; i < 12; i++ {
			fmt.Fprintf(&nodes, "%s%02d\t%s\t10\n", module, i, module)
		}
		for i := 11; i >= 0; i-- {
			for j := i - 1; j >= 0; j-- {
				fmt.Fprintf(&edges, "%s%02d\t%s%02d\t0.5\n", module, j, module, i)
			}
		}
	}
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&edges, "blue%02d\tpink%02d\t0.5\n", i, i)
	}
	for name, content := range map[string]string{"network_nodes.tsv": nodes.String(), "network_edges.tsv": edges.String()} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := buildNetworkIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestNetworkSamplingStableAndReadOnly(t *testing.T) {
	idx := networkFixture(t)
	before, _ := json.Marshal(idx.intraByGene)
	nodes, edges, err := sampleNetwork(idx, "combined", 12, 1, 2, 3, 3, true)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, e, err := sampleNetwork(idx, "combined", 12, 1, 2, 3, 3, true)
			if err != nil || !reflect.DeepEqual(nodes, n) || !reflect.DeepEqual(edges, e) {
				t.Error("same parameters changed sampled network")
			}
		}()
	}
	wg.Wait()
	after, _ := json.Marshal(idx.intraByGene)
	if string(before) != string(after) {
		t.Fatal("sampling mutated the shared network cache")
	}
}

func TestNetworkSamplingCountsOnlyVisibleNeighbors(t *testing.T) {
	// Each selected gene's strongest edge leads outside the visible sample.
	// The next eligible edge must still be selected with a per-gene budget of 1.
	idx := &networkIndex{
		nodes: []netNode{
			{ID: "a", Color: "pink", Kme: 1},
			{ID: "b", Color: "pink", Kme: 0.9},
			{ID: "hidden", Color: "pink", Kme: 0.1},
		},
		intraByGene: map[string][]netEdge{
			"a": {{Source: "a", Target: "hidden", Weight: 1, Type: "intra"}, {Source: "a", Target: "b", Weight: 0.5, Type: "intra"}},
			"b": {{Source: "b", Target: "hidden", Weight: 1, Type: "intra"}, {Source: "a", Target: "b", Weight: 0.5, Type: "intra"}},
		},
	}
	_, edges, err := sampleNetwork(idx, "intra", 2, 1, 1, 1, 1, true)
	if err != nil || len(edges) != 1 || edges[0].Source != "a" || edges[0].Target != "b" {
		t.Fatalf("expected visible a-b edge, got %+v (error %v)", edges, err)
	}
}
