package consent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

type todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

var client = &http.Client{Timeout: 10 * time.Second}

// baseURL reads the server URL from the environment.
// The same tests can run against any server just by changing this value.
func baseURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("AIP_INSTANCE_URL")
	if u == "" {
		t.Fatal("AIP_INSTANCE_URL is empty: set it to the server URL, e.g. http://localhost:8081")
	}
	return u
}

func do(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestServerIsReachable(t *testing.T) {
	resp := do(t, http.MethodGet, baseURL(t)+"/health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
}

func TestTodoCRUD(t *testing.T) {
	base := baseURL(t)
	var id int

	t.Run("create", func(t *testing.T) {
		resp := do(t, http.MethodPost, base+"/todos", todo{Title: "buy milk"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", resp.StatusCode)
		}
		var got todo
		json.NewDecoder(resp.Body).Decode(&got)
		if got.ID == 0 || got.Title != "buy milk" {
			t.Fatalf("unexpected todo: %+v", got)
		}
		id = got.ID
	})

	url := func() string { return fmt.Sprintf("%s/todos/%d", base, id) }

	t.Run("read", func(t *testing.T) {
		resp := do(t, http.MethodGet, url(), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var got todo
		json.NewDecoder(resp.Body).Decode(&got)
		if got.Title != "buy milk" || got.Done {
			t.Fatalf("unexpected todo: %+v", got)
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := do(t, http.MethodPut, url(), todo{Title: "buy milk", Done: true})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var got todo
		json.NewDecoder(resp.Body).Decode(&got)
		if !got.Done {
			t.Fatalf("done = false, want true: %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		resp := do(t, http.MethodDelete, url(), nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", resp.StatusCode)
		}
	})

	t.Run("read after delete is 404", func(t *testing.T) {
		resp := do(t, http.MethodGet, url(), nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}
