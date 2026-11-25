package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/regutrace"
)

func TestRegutraceHTTP(t *testing.T) {
	bin := os.Getenv("WGCNA_REGUTRACE")
	if bin == "" {
		t.Skip("set WGCNA_REGUTRACE to run the ReguTrace HTTP test")
	}
	genome := strings.Repeat("A", 3000)
	genome = genome[:150] + "ACGTAC" + genome[156:]
	genome = genome[:800] + "GTACGT" + genome[806:]
	gff := "##gff-version 3\n" +
		"Chr1\ttest\tgene\t201\t400\t.\t+\t.\tID=gene:G1;Name=GENE1\n" +
		"Chr1\ttest\tmRNA\t201\t400\t.\t+\t.\tID=transcript:G1.t1;Parent=gene:G1\n" +
		"Chr1\ttest\tgene\t601\t800\t.\t-\t.\tID=gene:G2;Name=GENE2\n" +
		"Chr1\ttest\tmRNA\t601\t800\t.\t-\t.\tID=transcript:G2.t1;Parent=gene:G2\n"
	pwm := ">MA0001.1 TEST\nA [ 10 0 0 0 10 0 ]\nC [ 0 10 0 0 0 10 ]\nG [ 0 0 10 0 0 0 ]\nT [ 0 0 0 10 0 0 ]\n"
	tfMap := "tf_id\ttf_symbol\tfamily\tmotif_id\treference_tf_id\nAT1G00010\tTF1\tHSF\tMA0001.1\tD1\n"
	tfGenes := "gene_id\ttf_symbol\tfamily\tidentification_status\nAT1G00010\tTF1\tHSF\tidentified\n"
	trace := `tf_id	motif_id	reference_donors
AT1G00010	MA0001.1	[{"tf_id":"D9","tf_symbol":"STALE"},{"tf_id":"D1","tf_symbol":"DONOR","tf_family":"HSF"}]
`

	mgr := regutrace.NewManager(t.TempDir(), bin)
	mux := http.NewServeMux()
	NewRegutraceHandler(mgr).Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	add := func(field, name, content string) {
		f, err := mw.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(f, content)
	}
	add("genome", "genome.fa", ">Chr1\n"+genome+"\n")
	add("gff", "annotation.gff3", gff)
	add("motifs", "motifs.jaspar", pwm)
	add("tf_map", "tf_map.tsv", tfMap)
	add("tf_genes", "tf_genes.tsv", tfGenes)
	add("trace", "tf_motif_trace.tsv", trace)
	_ = mw.Close()

	res, err := http.Post(srv.URL+"/api/toolkits/regutrace/jobs", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create: %s %s", res.Status, b)
	}
	var job regutrace.Job
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}

	params := `{"promoter_len":100,"downstream":100,"pvalue":0.001,"phase3_artifacts":true,"reuse_exact_pwm":true,"use_scan_cache":true}`
	runRes, err := http.Post(srv.URL+"/api/toolkits/regutrace/jobs/"+job.ID+"/run", "application/json", strings.NewReader(params))
	if err != nil {
		t.Fatal(err)
	}
	runRes.Body.Close()
	if runRes.StatusCode != http.StatusAccepted {
		t.Fatalf("run: %s", runRes.Status)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		getRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(getRes.Body).Decode(&job); err != nil {
			getRes.Body.Close()
			t.Fatal(err)
		}
		getRes.Body.Close()
		if job.Status == "succeeded" || job.Status == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if job.Status != "succeeded" {
		t.Fatalf("job = %+v", job)
	}

	qRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/query?direction=tf&q=TF1")
	if err != nil {
		t.Fatal(err)
	}
	defer qRes.Body.Close()
	if qRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(qRes.Body)
		t.Fatalf("query: %s %s", qRes.Status, b)
	}
	var q struct {
		Total int `json:"total"`
	}
	if err := json.NewDecoder(qRes.Body).Decode(&q); err != nil {
		t.Fatal(err)
	}
	if q.Total == 0 {
		t.Fatal("query returned no rows")
	}

	pRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/promoter?target=G1")
	if err != nil {
		t.Fatal(err)
	}
	defer pRes.Body.Close()
	if pRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(pRes.Body)
		t.Fatalf("promoter: %s %s", pRes.Status, b)
	}
	var view regutrace.PromoterView
	if err := json.NewDecoder(pRes.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if view.TargetID != "G1" || view.Sequence == "" || len(view.Hits) == 0 {
		t.Fatalf("view = %+v", view)
	}
	hit := view.Hits[0]
	if hit.PromoterStart == nil || hit.PromoterEnd == nil {
		t.Fatalf("hit missing promoter window: %+v", hit)
	}
	if len(hit.TFs) == 0 || len(hit.TFs[0].Donors) != 1 ||
		hit.TFs[0].Donors[0].TFID != "D1" || hit.TFs[0].Donors[0].TFSymbol != "DONOR" {
		t.Fatalf("hit donor = %+v", hit.TFs)
	}
	siteQuery := url.Values{
		"target":         {"G1"},
		"motif":          {hit.MotifID},
		"promoter_start": {fmt.Sprint(*hit.PromoterStart)},
		"promoter_end":   {fmt.Sprint(*hit.PromoterEnd)},
		"hit_start":      {fmt.Sprint(hit.Start)},
		"hit_end":        {fmt.Sprint(hit.End)},
		"strand":         {hit.Strand},
	}
	siteRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/site?" + siteQuery.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer siteRes.Body.Close()
	if siteRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(siteRes.Body)
		t.Fatalf("site: %s %s", siteRes.Status, b)
	}
	var site regutrace.SiteEvidenceResult
	if err := json.NewDecoder(siteRes.Body).Decode(&site); err != nil {
		t.Fatal(err)
	}
	if site.Total == 0 || len(site.Rows) == 0 || site.Rows[0].TFID == "" {
		t.Fatalf("site rows = %+v", site)
	}
	siteQuery.Set("promoter_start", "999")
	siteQuery.Set("promoter_end", "1200")
	wrongRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/site?" + siteQuery.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer wrongRes.Body.Close()
	if wrongRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(wrongRes.Body)
		t.Fatalf("wrong-window site: %s %s", wrongRes.Status, b)
	}
	var wrong regutrace.SiteEvidenceResult
	if err := json.NewDecoder(wrongRes.Body).Decode(&wrong); err != nil {
		t.Fatal(err)
	}
	if wrong.Total != 0 {
		t.Fatalf("wrong-window site rows = %+v", wrong)
	}

	// Phase 4: motif registry, sequence matrix, aggregated targets, enrichment,
	// provenance and a filtered prior export.
	motifRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/motifs?q=MA0001")
	if err != nil {
		t.Fatal(err)
	}
	var motifs regutrace.MotifListResult
	if err := json.NewDecoder(motifRes.Body).Decode(&motifs); err != nil {
		motifRes.Body.Close()
		t.Fatal(err)
	}
	motifRes.Body.Close()
	if motifs.Total == 0 || motifs.Rows[0].ID != "MA0001.1" || motifs.Rows[0].PWMHash == "" {
		t.Fatalf("motifs = %+v", motifs)
	}
	detailRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/motifs/MA0001.1")
	if err != nil {
		t.Fatal(err)
	}
	var detail regutrace.MotifDetail
	if err := json.NewDecoder(detailRes.Body).Decode(&detail); err != nil {
		detailRes.Body.Close()
		t.Fatal(err)
	}
	detailRes.Body.Close()
	if len(detail.Matrix) != 4 || len(detail.Matrix[0]) == 0 || len(detail.TFs) == 0 {
		t.Fatalf("motif detail = %+v", detail)
	}
	if detail.TestabilityStatus == "" || detail.TestabilityStatus == "not_computed" {
		t.Fatalf("testability status = %q", detail.TestabilityStatus)
	}
	targetsRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/targets?tf=AT1G00010&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	var targets regutrace.TFTargetResult
	if err := json.NewDecoder(targetsRes.Body).Decode(&targets); err != nil {
		targetsRes.Body.Close()
		t.Fatal(err)
	}
	targetsRes.Body.Close()
	if targets.Total == 0 || len(targets.Rows) == 0 || targets.Rows[0].SiteCount == 0 {
		t.Fatalf("targets = %+v", targets)
	}
	regRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/regulators?target=G1&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	var regulators regutrace.RegulatorResult
	if err := json.NewDecoder(regRes.Body).Decode(&regulators); err != nil {
		regRes.Body.Close()
		t.Fatal(err)
	}
	regRes.Body.Close()
	if regulators.Total == 0 || len(regulators.Rows) == 0 || regulators.Rows[0].TFID == "" {
		t.Fatalf("regulators = %+v", regulators)
	}
	enrichBody, _ := json.Marshal(regutrace.EnrichmentRequest{
		GeneSet: []string{"G1", "G2"}, Background: []string{"G1", "G2"}, MotifIDs: []string{"MA0001.1"},
	})
	enrichRes, err := http.Post(srv.URL+"/api/toolkits/regutrace/jobs/"+job.ID+"/enrich", "application/json", bytes.NewReader(enrichBody))
	if err != nil {
		t.Fatal(err)
	}
	var enrich regutrace.EnrichmentResult
	if err := json.NewDecoder(enrichRes.Body).Decode(&enrich); err != nil {
		enrichRes.Body.Close()
		t.Fatal(err)
	}
	enrichRes.Body.Close()
	if enrich.GeneSetSize != 2 || len(enrich.Rows) != 1 || len(enrich.Caveats) == 0 {
		t.Fatalf("enrichment = %+v", enrich)
	}
	provRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/provenance")
	if err != nil {
		t.Fatal(err)
	}
	var provenance regutrace.Provenance
	if err := json.NewDecoder(provRes.Body).Decode(&provenance); err != nil {
		provRes.Body.Close()
		t.Fatal(err)
	}
	provRes.Body.Close()
	if len(provenance.Inputs) < 2 || provenance.Inputs[0].SHA256 == "" {
		t.Fatalf("provenance = %+v", provenance)
	}
	priorRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/export?kind=prior&policy=strict")
	if err != nil {
		t.Fatal(err)
	}
	priorBody, _ := io.ReadAll(priorRes.Body)
	priorRes.Body.Close()
	if priorRes.StatusCode != http.StatusOK || !bytes.HasPrefix(priorBody, []byte("SQLite format 3")) {
		t.Fatalf("prior export: %s %q", priorRes.Status, priorBody[:min(32, len(priorBody))])
	}

	eRes, err := http.Get(srv.URL + "/api/toolkits/regutrace/jobs/" + job.ID + "/export?kind=evidence")
	if err != nil {
		t.Fatal(err)
	}
	defer eRes.Body.Close()
	if eRes.StatusCode != http.StatusOK {
		t.Fatalf("export: %s", eRes.Status)
	}
	b, _ := io.ReadAll(eRes.Body)
	if !bytes.Contains(b, []byte("prediction_motif")) {
		t.Fatalf("export body: %s", b)
	}
}
