package literature

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PubMed searches NCBI E-utilities (free, no key).
type PubMed struct {
	HTTP *http.Client
}

func (p *PubMed) Name() string { return "pubmed" }

func (p *PubMed) http() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

const eutils = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

func (p *PubMed) Search(query string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	ids, err := p.esearch(query, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return p.esummary(ids)
}

func (p *PubMed) esearch(query string, limit int) ([]string, error) {
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("term", query)
	q.Set("retmode", "json")
	q.Set("retmax", strconv.Itoa(limit))
	resp, err := p.http().Get(eutils + "/esearch.fcgi?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Result struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Result.IDList, nil
}

func (p *PubMed) esummary(ids []string) ([]Paper, error) {
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("id", strings.Join(ids, ","))
	q.Set("retmode", "json")
	resp, err := p.http().Get(eutils + "/esummary.fcgi?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	papers := make([]Paper, 0, len(ids))
	for _, id := range ids {
		raw, ok := out.Result[id]
		if !ok {
			continue
		}
		var r struct {
			Title   string `json:"title"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			Source  string `json:"source"`
			PubDate string `json:"pubdate"`
			ArticleIDs []struct {
				IDType string `json:"idtype"`
				Value  string `json:"value"`
			} `json:"articleids"`
			UID string `json:"uid"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		doi := ""
		for _, a := range r.ArticleIDs {
			if a.IDType == "doi" {
				doi = a.Value
			}
		}
		authors := make([]string, 0, len(r.Authors))
		for _, a := range r.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}
		year := 0
		if len(r.PubDate) >= 4 {
			year, _ = strconv.Atoi(r.PubDate[:4])
		}
		papers = append(papers, Paper{
			Title:   strings.TrimSpace(r.Title),
			Authors: authors,
			Year:    year,
			Journal: r.Source,
			PMID:    r.UID,
			DOI:     doi,
			URL:     "https://pubmed.ncbi.nlm.nih.gov/" + r.UID + "/",
			Source:  p.Name(),
		})
	}
	if len(papers) == 0 {
		return nil, fmt.Errorf("pubmed: no summaries returned")
	}
	return papers, nil
}
