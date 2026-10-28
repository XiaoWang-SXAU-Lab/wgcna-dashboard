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

// EuropePMC searches the Europe PMC REST API (free, no key).
type EuropePMC struct {
	HTTP *http.Client
}

func (p *EuropePMC) Name() string { return "europepmc" }

func (p *EuropePMC) http() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (p *EuropePMC) Search(query string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("format", "json")
	q.Set("resultType", "core")
	q.Set("pageSize", strconv.Itoa(limit))
	req, err := http.NewRequest(http.MethodGet,
		"https://www.ebi.ac.uk/europepmc/webservices/rest/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("europepmc %s", resp.Status)
	}
	var out struct {
		ResultList struct {
			Result []struct {
				Title        string `json:"title"`
				AuthorString string `json:"authorString"`
				PubYear      string `json:"pubYear"`
				JournalTitle string `json:"journalTitle"`
				PMID         string `json:"pmid"`
				DOI          string `json:"doi"`
				AbstractText string `json:"abstractText"`
				FullTextURLList struct {
					FullTextURL []struct {
						URL string `json:"url"`
					} `json:"fullTextUrl"`
				} `json:"fullTextUrlList"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	papers := make([]Paper, 0, len(out.ResultList.Result))
	for _, r := range out.ResultList.Result {
		u := ""
		if len(r.FullTextURLList.FullTextURL) > 0 {
			u = r.FullTextURLList.FullTextURL[0].URL
		}
		year, _ := strconv.Atoi(strings.TrimSpace(r.PubYear))
		papers = append(papers, Paper{
			Title:    r.Title,
			Authors:  splitAuthors(r.AuthorString),
			Year:     year,
			Journal:  r.JournalTitle,
			PMID:     r.PMID,
			DOI:      r.DOI,
			URL:      u,
			Abstract: r.AbstractText,
			Source:   p.Name(),
		})
	}
	return papers, nil
}

func splitAuthors(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
