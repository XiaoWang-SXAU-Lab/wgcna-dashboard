package api

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type reportNetworkSummary struct {
	Mode                  string
	Sampled, Nodes, Edges int
	File                  string
}

func keepReportEdge(list []netEdge, edge netEdge, limit int) []netEdge {
	if len(list) == limit && !networkEdgeLess(edge, list[len(list)-1]) {
		return list
	}
	i := sort.Search(len(list), func(i int) bool { return networkEdgeLess(edge, list[i]) })
	list = append(list, edge)
	copy(list[i+1:], list[i:len(list)-1])
	list[i] = edge
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

// Stream the full edge file with bounded top-K lists. A report should not load
// millions of full-network edges into a second in-memory visualization cache.
func reportNetworkSample(jobDir string) ([]netNode, []netEdge, error) {
	nodes, geneMod, err := readNetworkNodes(filepath.Join(jobDir, "network_nodes.tsv"))
	if err != nil {
		return nil, nil, err
	}
	kme := readKmeByGene(jobDir, geneMod)
	for i := range nodes {
		nodes[i].Kme = kme[nodes[i].ID]
	}
	selection := &networkIndex{nodes: nodes}
	sampled, _, err := sampleNetwork(selection, "intra", 2000, 20, 8, 20, 300, true)
	if err != nil {
		return nil, nil, err
	}
	idx := &networkIndex{nodes: sampled, intraByGene: map[string][]netEdge{}, interByPair: map[string][]netEdge{}}
	visible := map[string]int{}
	for i, node := range sampled {
		visible[node.ID] = i
	}
	f, err := os.Open(filepath.Join(jobDir, "network_edges.tsv"))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.ReuseRecord = true
	if _, err := r.Read(); err != nil {
		return nil, nil, err
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if len(record) < 3 {
			continue
		}
		a, b := record[0], record[1]
		ia, va := visible[a]
		ib, vb := visible[b]
		if !va && !vb {
			continue
		}
		weight, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return nil, nil, err
		}
		ma, mb := geneMod[a], geneMod[b]
		if ma == mb {
			if va {
				idx.nodes[ia].KWithin += weight
			}
			if vb {
				idx.nodes[ib].KWithin += weight
			}
		} else {
			if va {
				idx.nodes[ia].KCross += weight
				idx.nodes[ia].CrossDegree++
			}
			if vb {
				idx.nodes[ib].KCross += weight
				idx.nodes[ib].CrossDegree++
			}
		}
		if !va || !vb {
			continue
		}
		if ma == mb {
			edge := netEdge{Source: a, Target: b, Weight: weight, Type: "intra"}
			idx.intraByGene[a] = keepReportEdge(idx.intraByGene[a], edge, 8)
			idx.intraByGene[b] = keepReportEdge(idx.intraByGene[b], edge, 8)
		} else {
			if ma > mb {
				ma, mb = mb, ma
			}
			key := ma + "\x00" + mb
			idx.interByPair[key] = keepReportEdge(idx.interByPair[key], netEdge{Source: a, Target: b, Weight: weight, Type: "inter"}, 20)
		}
	}
	// All nodes were already sampled above; preserve that exact set here.
	return sampleNetwork(idx, "combined", 1<<30, 1, 8, 20, 300, true)
}
