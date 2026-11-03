package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	infra "github.com/lambdawalker/go.apexfission.aws.verifyemail/infrastructure"
	"github.com/lambdawalker/go.infra.composer/reporting"
)

func TestCLIJoinsAttestraVerificationCache(t *testing.T) {
	catalog, err := infra.Preview(nil, "dev")
	if err != nil {
		t.Fatal(err)
	}
	var live reporting.Artifact
	for _, a := range catalog {
		if a.Name == "email-signup" && a.Type == "aws:lambda/function:Function" {
			live = a
		}
	}
	if live.Key == "" {
		t.Fatal("actual native email-signup Lambda absent")
	}
	live.Key = "urn:pulumi:dev::attestra-application::aws:lambda/function:Function::email-signup"
	live.Feature = "email-verification"
	live.ProviderID = "attestra-email-signup-dev-real-instance"
	observed := time.Date(2026, 11, 2, 17, 42, 0, 0, time.UTC)
	live.Observation = reporting.Observation{Status: reporting.Present, Source: "pulumi-refresh", ObservedAt: observed}
	scope := reporting.Scope{Application: "attestra", Repository: "go.attestra.aws", Environment: "dev", Project: "attestra-application", Account: "123456789012", Region: "us-east-1", Backend: "s3://attestra-state", Stack: "dev"}
	cached := reporting.Report{SchemaVersion: reporting.SchemaVersion, Scope: scope, Artifacts: []reporting.Artifact{live}}
	dir := t.TempDir()
	cache := filepath.Join(dir, "app-report.json")
	f, err := os.Create(cache)
	if err != nil {
		t.Fatal(err)
	}
	err = reporting.WriteJSON(f, cached)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "preview")
	var stdout bytes.Buffer
	if err = run([]string{"-environment", "dev", "-cache", cache, "-project", scope.Project, "-account", scope.Account, "-backend", scope.Backend, "-output-dir", output}, &stdout); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result reporting.Report
	if err = json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	if !result.CacheLoaded || result.Scope != scope {
		t.Fatalf("source scope lost: %+v", result.Scope)
	}
	found := false
	for _, a := range result.Artifacts {
		if a.Name == "email-signup" && a.Type == live.Type {
			found = true
			if a.Feature != "email-verification" || a.ProviderID != live.ProviderID || a.Observation.Status != reporting.Present || a.Observation.Source != live.Observation.Source || !a.Observation.ObservedAt.Equal(observed) {
				t.Fatalf("evidence lost: %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("cached native Lambda absent")
	}
}
