// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/madarlevi/kolikaja/apps/api/internal/contract"
)

// Environment variables that enable the request tests.
const (
	envBaseURL     = "KOLIKAJA_API_URL"               // e.g. http://localhost:8080 (no /api/v1)
	envBearer      = "KOLIKAJA_API_TOKEN"             //nolint:gosec // #nosec G101 -- the name of an environment variable, not a credential.
	envAllowWrites = "KOLIKAJA_SECURITY_ALLOW_WRITES" // "1" to include POST/PUT/PATCH/DELETE
)

func loadEndpoints(t *testing.T) []Endpoint {
	t.Helper()
	path, err := contract.FindSpec(".")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return Endpoints(doc)
}

// TestEndpointInventory always runs: it proves that the endpoint list is generated from
// the contract and covers every operation.
func TestEndpointInventory(t *testing.T) {
	path, err := contract.FindSpec(".")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	endpoints := Endpoints(doc)
	if len(endpoints) != len(doc.Operations) || len(endpoints) == 0 {
		t.Fatalf("endpoints = %d, operations = %d", len(endpoints), len(doc.Operations))
	}
	withInputs := 0
	for _, e := range endpoints {
		if e.InjectionPoints() > 0 {
			withInputs++
		}
		for _, name := range e.Operation.PathParams() {
			if !contains(e.PathParams, name) {
				t.Errorf("%s: path parameter %s is not an injection point", e.Operation.ID, name)
			}
		}
		if e.Operation.BodySchema != nil {
			if _, ok := e.Body("x"); !ok {
				t.Errorf("%s: cannot build a request body", e.Operation.ID)
			}
		}
	}
	t.Logf("%d operations, %d with injection points, %d payloads", len(endpoints), withInputs, len(Payloads))
}

func TestPayloadList(t *testing.T) {
	seen := map[string]bool{}
	kinds := map[Kind]int{}
	for _, p := range Payloads {
		if p.Name == "" || p.Value == "" || seen[p.Name] {
			t.Errorf("invalid or duplicate payload %+v", p)
		}
		seen[p.Name] = true
		kinds[p.Kind]++
	}
	if kinds[SQLInjection] < 10 || kinds[XSS] < 8 {
		t.Errorf("payload coverage too small: %v", kinds)
	}
}

// TestInjectionPayloads sends every payload to every operation of a running API.
// Skipped unless KOLIKAJA_API_URL is set (phase 3 CI sets it against the Compose stack).
func TestInjectionPayloads(t *testing.T) {
	base := os.Getenv(envBaseURL)
	if base == "" {
		t.Skipf("%s is not set; the API is not running yet", envBaseURL)
	}
	allowWrites := os.Getenv(envAllowWrites) == "1"
	token := os.Getenv(envBearer)
	client := &http.Client{Timeout: 15 * time.Second}

	for _, e := range loadEndpoints(t) {
		if e.Unsafe() && !allowWrites {
			continue
		}
		t.Run(e.Operation.ID, func(t *testing.T) {
			for _, p := range Payloads {
				probe(t, client, base, token, e, p)
			}
		})
	}
}

func probe(t *testing.T, client *http.Client, base, token string, e Endpoint, p Payload) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var body io.Reader
	if data, ok := e.Body(p.Value); ok {
		body = bytes.NewReader(data)
	}
	// The target is the API under test, configured by the tester through KOLIKAJA_API_URL;
	// sending crafted requests to it is the purpose of this harness.
	req, err := http.NewRequestWithContext(ctx, e.Operation.Method, e.URL(base, p.Value), body) //nolint:gosec // #nosec G704 -- intentional request to the configured test target.
	if err != nil {
		// Some payloads cannot form a valid URL; that is a client-side rejection, not a finding.
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, err := client.Do(req) //nolint:gosec // #nosec G704 -- intentional request to the configured test target.
	if err != nil {
		t.Errorf("%s %s: request failed: %v", p.Kind, p.Name, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	elapsed := time.Since(start)
	text := string(data)

	if resp.StatusCode >= 500 {
		t.Errorf("%s %s: status %d (payload must be rejected as a client error)", p.Kind, p.Name, resp.StatusCode)
	}
	for _, marker := range SQLErrorMarkers {
		if strings.Contains(text, marker) {
			t.Errorf("%s %s: response leaks internal detail %q", p.Kind, p.Name, marker)
		}
	}
	if p.Name == "time-based" && elapsed > 4*time.Second {
		t.Errorf("%s %s: response took %s; possible time-based SQL injection", p.Kind, p.Name, elapsed)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("%s %s: missing X-Content-Type-Options: nosniff", p.Kind, p.Name)
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		t.Errorf("%s %s: API answered with HTML (%s); reflected payloads could execute", p.Kind, p.Name, ct)
	}
	if p.Kind == XSS && !strings.Contains(ct, "json") && strings.Contains(text, p.Value) {
		t.Errorf("%s %s: payload reflected verbatim in a non-JSON response", p.Kind, p.Name)
	}
	if resp.StatusCode >= 400 && strings.Contains(ct, "json") {
		var envelope struct {
			Error *struct {
				Code      string `json:"code"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil || envelope.Error == nil || envelope.Error.Code == "" {
			t.Errorf("%s %s: status %d without the error envelope", p.Kind, p.Name, resp.StatusCode)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
