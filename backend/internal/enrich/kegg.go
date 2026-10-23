package enrich

import "strings"

// Pathway is one KO -> pathway association from a TBtools kingdom background.
type Pathway struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"` // A-category, e.g. "A09100 Metabolism"
}

// KEGGBackground holds a kingdom's KO -> pathway map.
type KEGGBackground struct {
	KO2Pathway map[string][]Pathway
}

// ParseTBtoolsKEGG parses a TBtoolsKeggBackEnd file (no header):
//
//	KO \t KO name \t "pathwayID pathwayName" \t Brite \t A-category
func ParseTBtoolsKEGG(data []byte) (*KEGGBackground, error) {
	_, rows, err := readTable(data, false)
	if err != nil {
		return nil, err
	}
	bg := &KEGGBackground{KO2Pathway: map[string][]Pathway{}}
	for _, r := range rows {
		if len(r) < 3 {
			continue
		}
		ko := strings.TrimSpace(r[0])
		id, name := splitPath(r[2])
		if ko == "" || id == "" {
			continue
		}
		cat := ""
		if len(r) >= 5 {
			cat = strings.TrimSpace(r[4])
		}
		bg.KO2Pathway[ko] = append(bg.KO2Pathway[ko], Pathway{ID: id, Name: name, Category: cat})
	}
	return bg, nil
}

func splitPath(s string) (id, name string) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, " ", 2)
	id = parts[0]
	if len(parts) == 2 {
		name = strings.TrimSpace(parts[1])
	}
	return id, name
}

// BuildBackground maps gene -> KO -> pathway through the kingdom background.
// Only genes with at least one valid kingdom pathway enter the universe.
func (b *KEGGBackground) BuildBackground(gene2ko map[string][]string) Background {
	bg := Background{Gene2Term: map[string][]string{}, TermName: map[string]string{}}
	uni := map[string]bool{}
	for g, kos := range gene2ko {
		seen := map[string]bool{}
		for _, ko := range kos {
			for _, pw := range b.KO2Pathway[ko] {
				if seen[pw.ID] {
					continue
				}
				seen[pw.ID] = true
				bg.Gene2Term[g] = append(bg.Gene2Term[g], pw.ID)
				if bg.TermName[pw.ID] == "" {
					bg.TermName[pw.ID] = pw.Name
				}
			}
		}
		if len(bg.Gene2Term[g]) > 0 {
			uni[g] = true
		}
	}
	bg.Universe = sortedKeys(uni)
	return bg
}
