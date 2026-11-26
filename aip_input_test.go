package consent

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"
)

// These tests are PoC experiments. They only run when the extra input is given,
// so a normal run just skips them.

// describe says whether a value is set, without ever printing a secret.
func describe(name string) string {
	v := os.Getenv(name)
	if v == "" {
		return name + ": not set"
	}
	return name + ": set (length " + strconv.Itoa(len(v)) + ")"
}

// TestExtraInput checks that separate Jenkins parameters arrive as env vars.
func TestExtraInput(t *testing.T) {
	names := []string{"AIP_ENVIRONMENT", "AIP_CLIENT_ID", "AIP_CLIENT_SECRET"}
	given := false
	for _, n := range names {
		if os.Getenv(n) != "" {
			given = true
		}
	}
	if !given {
		t.Skip("no extra input given")
	}
	for _, n := range names {
		t.Log(describe(n)) // only "set" and the length, never the value
	}
	if os.Getenv("AIP_ENVIRONMENT") == "" {
		t.Error("AIP_ENVIRONMENT is empty")
	}
}

// TestInputJSON checks that a whole JSON document can travel in one parameter.
func TestInputJSON(t *testing.T) {
	raw := os.Getenv("AIP_INPUT_JSON")
	if raw == "" {
		t.Skip("AIP_INPUT_JSON not given")
	}
	var in struct {
		InstanceURL  string `json:"instance_url"`
		Environment  string `json:"environment"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("AIP_INPUT_JSON is not valid JSON: %v", err)
	}
	t.Logf("instance_url=%q environment=%q client_id set=%v client_secret set=%v",
		in.InstanceURL, in.Environment, in.ClientID != "", in.ClientSecret != "")
	if in.InstanceURL == "" {
		t.Error("instance_url is missing in the JSON")
	}
}

// TestSimulatedHang sleeps for a long time, to simulate a build that never finishes.
// It does nothing unless AIP_SIMULATE_HANG is "true".
func TestSimulatedHang(t *testing.T) {
	if os.Getenv("AIP_SIMULATE_HANG") != "true" {
		t.Skip("AIP_SIMULATE_HANG is not true")
	}
	t.Log("simulating a stuck test: sleeping 5 minutes")
	time.Sleep(5 * time.Minute)
}
