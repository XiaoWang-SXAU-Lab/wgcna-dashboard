package literature

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SemanticScholar searches the Semantic Scholar Graph API (free, no key).
type SemanticScholar struct {
	HTTP *http.Client
}

func (p *SemanticScholar) Name() string { return "semantic" }

func (p *SemanticScholar) http() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (p *SemanticScholar) Search(query string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("fields", "title,abstract,year,authors,externalIds,url,venue")
	req, err := http.NewRequest(http.MethodGet,
		"https://api.semanticscholar.org/graph/v1/paper/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semantic scholar %s", resp.Status)
	}
	var out struct {
		Data []struct {
			Title    string `json:"title"`
			Abstract string `json:"abstract"`
			Year     int    `json:"year"`
			Authors  []struct {
				Name string `json:"name"`
			} `json:"authors"`
			ExternalIDs struct {
				DOI  string `json:"DOI"`
				PMID string `json:"PMID"`
			} `json:"externalIds"`
			URL   string `json:"url"`
			Venue string `json:"venue"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	papers := make([]Paper, 0, len(out.Data))
	for _, d := range out.Data {
		authors := make([]string, 0, len(d.Authors))
		for _, a := range d.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}
		papers = append(papers, Paper{
			Title:    d.Title,
			Authors:  authors,
			Year:     d.Year,
			Journal:  d.Venue,
			PMID:     d.ExternalIDs.PMID,
			DOI:      d.ExternalIDs.DOI,
			URL:      d.URL,
			Abstract: d.Abstract,
			Source:   p.Name(),
		})
	}
	return papers, nil
}
