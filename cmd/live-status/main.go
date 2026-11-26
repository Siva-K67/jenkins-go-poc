// live-status watches a Jenkins build and prints the status of every Go test
// (RUNNING, PASS, FAIL, SKIP) while the build is still running.
//
// It reads the build's console log from the Jenkins REST API. This is a PoC for
// what the Testing Server would do. It reads the login from environment variables
// so the token is never typed on the command line or saved in the repo:
//
//	JENKINS_USER, JENKINS_TOKEN, and optionally JENKINS_URL.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	BaseURL  string
	Job      string
	Build    int // 0 means the latest build
	User     string
	Token    string
	Interval time.Duration
}

// fatalError is an error that retrying will not fix (wrong login, build missing).
type fatalError struct{ msg string }

func (e fatalError) Error() string { return e.msg }

func main() {
	var cfg config
	flag.StringVar(&cfg.BaseURL, "url", envOr("JENKINS_URL", "http://localhost:8080"), "Jenkins base URL")
	flag.StringVar(&cfg.Job, "job", "go-test-poc", "Jenkins job name")
	flag.IntVar(&cfg.Build, "build", 0, "build number (0 = latest build)")
	flag.DurationVar(&cfg.Interval, "interval", 2*time.Second, "how often to read the log")
	serveMode := flag.Bool("serve", false, "show a live web page instead of printing in the terminal")
	addr := flag.String("addr", "127.0.0.1:9000", "address of the web page (used with -serve)")
	flag.Parse()
	cfg.User = os.Getenv("JENKINS_USER")
	cfg.Token = os.Getenv("JENKINS_TOKEN")

	if cfg.User == "" || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "JENKINS_USER and JENKINS_TOKEN must be set (see README)")
		os.Exit(2)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	if *serveMode {
		if err := serve(client, cfg, *addr, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		return
	}
	failed, err := watch(client, cfg, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// get calls the Jenkins API with the login and turns bad answers into clear errors.
func get(client *http.Client, cfg config, path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, fatalError{err.Error()}
	}
	req.SetBasicAuth(cfg.User, cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Jenkins: %w", err) // maybe Jenkins is down: worth a retry
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		return resp, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		resp.Body.Close()
		return nil, fatalError{fmt.Sprintf("Jenkins rejected the login (HTTP %d): check JENKINS_USER and JENKINS_TOKEN", resp.StatusCode)}
	case resp.StatusCode == http.StatusNotFound:
		resp.Body.Close()
		return nil, fatalError{fmt.Sprintf("not found (HTTP 404): %s", path)}
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP %d from Jenkins", resp.StatusCode)
	}
}

// getRetry tries a few times, but gives up straight away on errors that will not get better.
func getRetry(client *http.Client, cfg config, path string) (*http.Response, error) {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		var resp *http.Response
		resp, err = get(client, cfg, path)
		if err == nil {
			return resp, nil
		}
		var fe fatalError
		if errors.As(err, &fe) {
			return nil, err
		}
		time.Sleep(cfg.Interval)
	}
	return nil, err
}

func latestBuild(client *http.Client, cfg config) (int, error) {
	resp, err := getRetry(client, cfg, "/job/"+cfg.Job+"/lastBuild/api/json?tree=number")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var v struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return 0, err
	}
	return v.Number, nil
}

// watch follows one build until its log is complete and returns the number of failed tests.
func watch(client *http.Client, cfg config, out io.Writer) (int, error) {
	build := cfg.Build
	if build == 0 {
		n, err := latestBuild(client, cfg)
		if err != nil {
			return 0, err
		}
		build = n
	}
	fmt.Fprintf(out, "Watching %s build #%d\n\n", cfg.Job, build)

	tr := newTracker()
	err := follow(client, cfg, build, func(chunk string) {
		for _, e := range tr.Feed(chunk) {
			fmt.Fprintf(out, "%s  %-8s %s\n", time.Now().Format("15:04:05"), e.Status, e.Name)
		}
	})
	if err != nil {
		return 0, err
	}
	return tr.Summary(out), nil
}

// follow reads a build's console log piece by piece and passes each piece to handle.
// It returns when Jenkins says the log is complete (the build is over).
func follow(client *http.Client, cfg config, build int, handle func(chunk string)) error {
	offset := 0
	for {
		path := fmt.Sprintf("/job/%s/%d/logText/progressiveText?start=%d", cfg.Job, build, offset)
		resp, err := getRetry(client, cfg, path)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		handle(string(body))

		// Jenkins tells us where to continue, and whether more log is still coming.
		if next, err := strconv.Atoi(resp.Header.Get("X-Text-Size")); err == nil {
			offset = next
		}
		if resp.Header.Get("X-More-Data") != "true" {
			return nil
		}
		time.Sleep(cfg.Interval)
	}
}
