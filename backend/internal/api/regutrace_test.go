package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	tfMap := "tf_id\ttf_symbol\tfamily\tmotif_id\nAT1G00010\tTF1\tHSF\tMA0001.1\n"

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

	params := `{"promoter_len":100,"downstream":100,"pvalue":0.001}`
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
