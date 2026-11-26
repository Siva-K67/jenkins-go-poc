package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// States of what we are watching.
const (
	stateWaiting  = "waiting"  // no build found yet
	stateRunning  = "running"  // reading the log of a build
	stateFinished = "finished" // the log is complete
	stateError    = "error"    // we could not read from Jenkins
)

// liveState is what the web page shows. The Jenkins reader writes to it and
// the web server reads from it, so everything goes through the mutex.
type liveState struct {
	mu    sync.Mutex
	build int
	state string
	err   string
	tr    *tracker
}

// statusJSON is the answer of /api/status. A Testing Server could read the same JSON.
type statusJSON struct {
	Build   int     `json:"build"`
	State   string  `json:"state"`
	Error   string  `json:"error,omitempty"`
	Total   int     `json:"total"`
	Pass    int     `json:"pass"`
	Fail    int     `json:"fail"`
	Skip    int     `json:"skip"`
	Running int     `json:"running"`
	Tests   []Event `json:"tests"`
}

func newLiveState() *liveState {
	return &liveState{state: stateWaiting, tr: newTracker()}
}

// reset starts watching a new build with an empty table.
func (ls *liveState) reset(build int) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.build, ls.state, ls.err, ls.tr = build, stateRunning, "", newTracker()
}

func (ls *liveState) feed(chunk string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.tr.Feed(chunk)
}

func (ls *liveState) setState(state, errMsg string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.state, ls.err = state, errMsg
}

func (ls *liveState) snapshot() statusJSON {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	s := statusJSON{Build: ls.build, State: ls.state, Error: ls.err, Tests: ls.tr.Snapshot()}
	for _, e := range s.Tests {
		switch e.Status {
		case Pass:
			s.Pass++
		case Fail:
			s.Fail++
		case Skip:
			s.Skip++
		case Running:
			s.Running++
		}
	}
	s.Total = len(s.Tests)
	return s
}

// run keeps the state up to date. With a fixed build number it follows that one build.
// Without one it follows the latest build and switches when a newer build starts,
// so the page can stay open while you start build after build.
func (ls *liveState) run(client *http.Client, cfg config) {
	current := 0
	for {
		build := cfg.Build
		if build == 0 {
			n, err := latestBuild(client, cfg)
			if err != nil {
				ls.setState(stateError, err.Error())
				time.Sleep(cfg.Interval)
				continue
			}
			build = n
		}
		if build != current {
			current = build
			ls.reset(build)
			if err := follow(client, cfg, build, ls.feed); err != nil {
				ls.setState(stateError, err.Error())
				current = 0 // try this build again
			} else {
				ls.setState(stateFinished, "")
			}
		}
		if cfg.Build != 0 {
			return // a fixed build has nothing more to wait for
		}
		time.Sleep(cfg.Interval)
	}
}

func (ls *liveState) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(ls.snapshot())
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, pageHTML)
}

// serve starts the web page. It listens on the local machine only by default,
// because the page is for you and the Jenkins login stays inside this program.
func serve(client *http.Client, cfg config, addr string, out io.Writer) error {
	ls := newLiveState()
	go ls.run(client, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", ls.handleStatus)
	mux.HandleFunc("/", handlePage)

	fmt.Fprintf(out, "Live test status for %s: open http://%s in your browser (Ctrl+C to stop)\n", cfg.Job, addr)
	return http.ListenAndServe(addr, mux)
}

// The whole page is in one string, so there is nothing else to install or download.
// Test names come from the log, so the script writes them with textContent and never innerHTML.
const pageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Live Test Status</title>
<style>
  :root {
    --bg: #ffffff; --fg: #1a1a1a; --muted: #5f6368; --line: #dadce0;
    --pass: #137333; --pass-bg: #e6f4ea;
    --fail: #b3261e; --fail-bg: #fce8e6;
    --run: #1a56c4; --run-bg: #e8f0fe;
    --skip: #5f6368; --skip-bg: #f1f3f4;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #16181c; --fg: #e8eaed; --muted: #9aa0a6; --line: #3c4043;
      --pass: #81c995; --pass-bg: #1e3a29;
      --fail: #f28b82; --fail-bg: #4a1f1c;
      --run: #8ab4f8; --run-bg: #1c2e4a;
      --skip: #9aa0a6; --skip-bg: #2a2d31;
    }
  }
  body { margin: 0; padding: 16px; background: var(--bg); color: var(--fg);
         font: 15px/1.5 system-ui, -apple-system, Segoe UI, Roboto, sans-serif; }
  main { max-width: 860px; margin: 0 auto; }
  h1 { font-size: 20px; margin: 8px 0 4px; }
  #head { color: var(--muted); margin: 0 0 12px; }
  #counts { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
  .chip { border-radius: 999px; padding: 2px 12px; font-size: 13px; font-weight: 600; }
  table { width: 100%; border-collapse: collapse; }
  th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid var(--line); }
  th { color: var(--muted); font-size: 13px; font-weight: 600; }
  td.name { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; word-break: break-all; }
  .PASS { color: var(--pass); background: var(--pass-bg); }
  .FAIL { color: var(--fail); background: var(--fail-bg); }
  .RUNNING { color: var(--run); background: var(--run-bg); }
  .SKIP { color: var(--skip); background: var(--skip-bg); }
  .error { color: var(--fail); }
</style>
</head>
<body>
<main>
  <h1>Live test status</h1>
  <p id="head">Waiting for a build...</p>
  <div id="counts"></div>
  <table>
    <thead><tr><th>Test</th><th>Status</th><th>Time</th></tr></thead>
    <tbody id="rows"></tbody>
  </table>
</main>
<script>
function chip(text, cls) {
  var s = document.createElement('span');
  s.className = 'chip ' + cls;
  s.textContent = text;
  return s;
}
function render(s) {
  var head = document.getElementById('head');
  head.className = s.error ? 'error' : '';
  if (s.state === 'waiting') head.textContent = 'Waiting for a build...';
  else if (s.error) head.textContent = 'Build #' + s.build + ' - problem: ' + s.error;
  else head.textContent = 'Build #' + s.build + ' - ' + s.state;

  var counts = document.getElementById('counts');
  counts.replaceChildren(
    chip('total ' + s.total, 'SKIP'), chip('running ' + s.running, 'RUNNING'),
    chip('pass ' + s.pass, 'PASS'), chip('fail ' + s.fail, 'FAIL'), chip('skip ' + s.skip, 'SKIP'));

  var rows = document.getElementById('rows');
  rows.replaceChildren();
  (s.tests || []).forEach(function (t) {
    var tr = document.createElement('tr');
    var name = document.createElement('td'); name.className = 'name'; name.textContent = t.name;
    var st = document.createElement('td'); st.appendChild(chip(t.status, t.status));
    var time = document.createElement('td'); time.textContent = t.duration || '';
    tr.append(name, st, time);
    rows.appendChild(tr);
  });
}
function tick() {
  fetch('/api/status', { cache: 'no-store' })
    .then(function (r) { return r.json(); })
    .then(render)
    .catch(function () {
      var head = document.getElementById('head');
      head.className = 'error';
      head.textContent = 'Cannot reach the live-status program. Is it still running?';
    });
}
tick();
setInterval(tick, 1000);
</script>
</body>
</html>
`
