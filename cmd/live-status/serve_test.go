package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStatusEndpoint(t *testing.T) {
	ls := newLiveState()
	ls.reset(7)
	ls.feed(sampleLog)

	rec := httptest.NewRecorder()
	ls.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	var got statusJSON
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Build != 7 || got.State != stateRunning {
		t.Errorf("build/state = %d/%s, want 7/running", got.Build, got.State)
	}
	// sampleLog has 4 tests: 2 passed, 2 failed.
	if got.Total != 4 || got.Pass != 2 || got.Fail != 2 || got.Running != 0 {
		t.Errorf("counts = %+v", got)
	}
	if len(got.Tests) != 4 || got.Tests[0].Name != "TestServerIsReachable" || got.Tests[0].Status != Pass {
		t.Errorf("tests = %+v", got.Tests)
	}
}

func TestPageIsServed(t *testing.T) {
	rec := httptest.NewRecorder()
	handlePage(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Live test status") {
		t.Errorf("page: code %d, body missing title", rec.Code)
	}

	rec = httptest.NewRecorder()
	handlePage(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown path: code %d, want 404", rec.Code)
	}
}

// Runs the reader against a fake Jenkins and checks that the page state ends up finished.
func TestRunFillsState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/job/go-test-poc/7/logText/progressiveText" {
			w.Header().Set("X-Text-Size", fmt.Sprint(len(sampleLog)))
			w.Header().Set("X-More-Data", "false")
			fmt.Fprint(w, sampleLog)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	ls := newLiveState()
	cfg := config{BaseURL: srv.URL, Job: "go-test-poc", Build: 7, User: "u", Token: "t", Interval: time.Millisecond}
	ls.run(srv.Client(), cfg) // a fixed build returns when the log is complete

	got := ls.snapshot()
	if got.State != stateFinished || got.Build != 7 || got.Total != 4 {
		t.Errorf("snapshot = %+v", got)
	}
}

func TestRunReportsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	ls := newLiveState()
	cfg := config{BaseURL: srv.URL, Job: "go-test-poc", Build: 7, User: "u", Token: "t", Interval: time.Millisecond}
	ls.run(srv.Client(), cfg)

	got := ls.snapshot()
	if got.State != stateError || !strings.Contains(got.Error, "rejected the login") {
		t.Errorf("snapshot = %+v", got)
	}
}
