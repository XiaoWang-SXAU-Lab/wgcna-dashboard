package literature

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/netutil"
)

// Scholar searches Google Scholar via SerpAPI (requires the user's API key).
// It is only enabled when a key is provided.
type Scholar struct {
	APIKey string
	HTTP   *http.Client
}

func (p *Scholar) Name() string { return "scholar" }

func (p *Scholar) http() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return netutil.Client(20 * time.Second)
}

func (p *Scholar) Search(query string, limit int) ([]Paper, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("google scholar: no API key configured")
	}
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("engine", "google_scholar")
	q.Set("q", query)
	q.Set("api_key", p.APIKey)
	q.Set("num", strconv.Itoa(limit))
	resp, err := p.http().Get("https://serpapi.com/search.json?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serpapi %s", resp.Status)
	}
	var out struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			PubInfo struct {
				Summary string `json:"summary"`
			} `json:"publication_info"`
		} `json:"organic_results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	papers := make([]Paper, 0, len(out.OrganicResults))
	for _, r := range out.OrganicResults {
		year := 0
		// publication_info.summary usually ends with the year, e.g. "... - Nature, 2021"
		if i := strings.LastIndex(r.PubInfo.Summary, ","); i >= 0 {
			if y, err := strconv.Atoi(strings.TrimSpace(r.PubInfo.Summary[i+1:])); err == nil {
				year = y
			}
		}
		papers = append(papers, Paper{
			Title:    r.Title,
			Year:     year,
			URL:      r.Link,
			Abstract: r.Snippet,
			Source:   p.Name(),
		})
	}
	return papers, nil
}
