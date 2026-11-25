package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Jenkins hides small markers (ha:////...) inside the raw console log.
// A browser does not show them, but the API does, so we remove them first.
var markerRe = regexp.MustCompile(`ha:////[A-Za-z0-9+/]+={0,2}`)

var (
	runRe    = regexp.MustCompile(`^\s*=== RUN\s+(\S+)\s*$`)
	resultRe = regexp.MustCompile(`^\s*--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+s)\)`)
)

// Statuses a test can have.
const (
	Running = "RUNNING"
	Pass    = "PASS"
	Fail    = "FAIL"
	Skip    = "SKIP"
)

// Event is one change in a test's status.
type Event struct {
	Name     string
	Status   string
	Duration string
}

// tracker turns console log text into test statuses.
// Feed it the log in any size of pieces. It remembers a half line until the rest arrives.
type tracker struct {
	partial string
	order   []string
	tests   map[string]*Event
}

func newTracker() *tracker {
	return &tracker{tests: map[string]*Event{}}
}

// Feed reads a new piece of the log and returns the status changes found in it.
func (t *tracker) Feed(chunk string) []Event {
	data := t.partial + chunk
	lines := strings.Split(data, "\n")
	// The last piece has no line ending yet, so keep it for the next call.
	t.partial = lines[len(lines)-1]
	lines = lines[:len(lines)-1]

	var events []Event
	for _, line := range lines {
		line = strings.TrimRight(markerRe.ReplaceAllString(line, ""), "\r")
		if m := runRe.FindStringSubmatch(line); m != nil {
			events = append(events, t.set(m[1], Running, ""))
		} else if m := resultRe.FindStringSubmatch(line); m != nil {
			events = append(events, t.set(m[2], m[1], m[3]))
		}
	}
	return events
}

func (t *tracker) set(name, status, duration string) Event {
	e, ok := t.tests[name]
	if !ok {
		e = &Event{Name: name}
		t.tests[name] = e
		t.order = append(t.order, name)
	}
	e.Status = status
	if duration != "" {
		e.Duration = duration
	}
	return *e
}

// Summary prints a table of every test and returns how many failed.
func (t *tracker) Summary(out io.Writer) int {
	counts := map[string]int{}
	fmt.Fprintf(out, "\n%-45s %-8s %s\n", "TEST", "STATUS", "TIME")
	for _, name := range t.order {
		e := t.tests[name]
		counts[e.Status]++
		fmt.Fprintf(out, "%-45s %-8s %s\n", e.Name, e.Status, e.Duration)
	}
	fmt.Fprintf(out, "\ntotal=%d pass=%d fail=%d skip=%d running=%d\n",
		len(t.order), counts[Pass], counts[Fail], counts[Skip], counts[Running])
	return counts[Fail]
}
