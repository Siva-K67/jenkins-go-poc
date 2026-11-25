package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A piece of a real Jenkins console log, including the hidden ha://// markers.
const sampleLog = "Started by user ha:////4Hcbe+/AAAA==Siva Kumar\n" +
	"[Pipeline] sh\n" +
	"=== RUN   TestServerIsReachable\n" +
	"--- PASS: TestServerIsReachable (2.01s)\n" +
	"=== RUN   TestTodoCRUD\n" +
	"=== RUN   TestTodoCRUD/create\n" +
	"ha:////4KMgS5+wddo/AAA=    --- PASS: TestTodoCRUD/create (2.00s)\n" +
	"=== RUN   TestTodoCRUD/read\n" +
	"    --- FAIL: TestTodoCRUD/read (2.00s)\n" +
	"--- FAIL: TestTodoCRUD (4.01s)\n"

func TestTrackerReadsStatuses(t *testing.T) {
	tr := newTracker()
	events := tr.Feed(sampleLog)

	want := []string{
		"RUNNING TestServerIsReachable",
		"PASS TestServerIsReachable",
		"RUNNING TestTodoCRUD",
		"RUNNING TestTodoCRUD/create",
		"PASS TestTodoCRUD/create",
		"RUNNING TestTodoCRUD/read",
		"FAIL TestTodoCRUD/read",
		"FAIL TestTodoCRUD",
	}
	var got []string
	for _, e := range events {
		got = append(got, e.Status+" "+e.Name)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("events:\n got  %v\n want %v", got, want)
	}

	var out bytes.Buffer
	if failed := tr.Summary(&out); failed != 2 {
		t.Fatalf("failed = %d, want 2\n%s", failed, out.String())
	}
}

// A log line may be cut in the middle between two reads. It must still be read correctly.
func TestTrackerHandlesSplitLines(t *testing.T) {
	tr := newTracker()
	first := tr.Feed("=== RUN   TestA\n--- PASS: Tes")
	second := tr.Feed("tA (1.00s)\n")
	if len(first) != 1 || first[0].Status != Running {
		t.Fatalf("first = %+v", first)
	}
	if len(second) != 1 || second[0].Status != Pass || second[0].Duration != "1.00s" {
		t.Fatalf("second = %+v", second)
	}
}

// Runs watch() against a fake Jenkins that hands out the log in two pieces.
func TestWatchFollowsProgressiveLog(t *testing.T) {
	cut := strings.Index(sampleLog, "=== RUN   TestTodoCRUD\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "siva" || p != "secret" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/job/go-test-poc/lastBuild/api/json":
			fmt.Fprint(w, `{"number":7}`)
		case "/job/go-test-poc/7/logText/progressiveText":
			start, _ := strconv.Atoi(r.URL.Query().Get("start"))
			end, more := len(sampleLog), "false"
			if start < cut {
				end, more = cut, "true"
			}
			w.Header().Set("X-Text-Size", strconv.Itoa(end))
			w.Header().Set("X-More-Data", more)
			fmt.Fprint(w, sampleLog[start:end])
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := config{BaseURL: srv.URL, Job: "go-test-poc", User: "siva", Token: "secret", Interval: time.Millisecond}
	var out bytes.Buffer
	failed, err := watch(srv.Client(), cfg, &out)
	if err != nil {
		t.Fatal(err)
	}
	if failed != 2 {
		t.Fatalf("failed = %d, want 2\n%s", failed, out.String())
	}
	for _, s := range []string{"build #7", "RUNNING  TestServerIsReachable", "PASS     TestTodoCRUD/create", "total=4"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output is missing %q:\n%s", s, out.String())
		}
	}
}

func TestWatchWrongLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := config{BaseURL: srv.URL, Job: "go-test-poc", User: "x", Token: "y", Interval: time.Millisecond}
	_, err := watch(srv.Client(), cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "rejected the login") {
		t.Fatalf("err = %v, want a login error", err)
	}
}
