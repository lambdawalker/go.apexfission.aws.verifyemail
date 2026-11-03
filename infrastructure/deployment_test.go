package infrastructure

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeConfig map[string]string

func (c fakeConfig) Get(k string) string     { return c[k] }
func (c fakeConfig) Require(k string) string { return c[k] }
func (c fakeConfig) RequireSecret(k string) pulumi.StringOutput {
	return pulumi.ToSecret(c[k]).(pulumi.StringOutput)
}

type mocks struct {
	mu        sync.Mutex
	resources map[string]pulumi.MockResourceArgs
}

func (m *mocks) Call(a pulumi.MockCallArgs) (resource.PropertyMap, error) { return a.Args, nil }
func (m *mocks) NewResource(a pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[a.Name] = a
	o := a.Inputs.Copy()
	for k, val := range map[string]string{"name": a.Name, "arn": "arn:" + a.Name, "versionId": "version-1", "invokeArn": "arn:invoke:" + a.Name} {
		o[resource.PropertyKey(k)] = resource.NewStringProperty(val)
	}
	return a.Name + "-id", o, nil
}
func fixtures(t *testing.T) (fakeConfig, Options) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"signup", "resend", "confirm"} {
		if err := os.WriteFile(filepath.Join(dir, n+".zip"), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	in := map[string]pulumi.StringInput{}
	for _, k := range integrationFields() {
		in[k] = pulumi.String("provided-" + k)
	}
	return fakeConfig{"appOrigin": "https://app.example.test", "senderDomain": "example.test", "senderAddress": "verify@example.test", "proofKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}, Options{ArtifactDir: dir, Environment: "dev", Integration: in}
}
func TestPreflightRejectsMissingIntegrationAndArchivesWithoutResources(t *testing.T) {
	for _, failure := range []string{"binding", "archive", "secret"} {
		t.Run(failure, func(t *testing.T) {
			cfg, opts := fixtures(t)
			switch failure {
			case "binding":
				delete(opts.Integration, "grants_table_arn")
			case "archive":
				os.Remove(filepath.Join(opts.ArtifactDir, "confirm.zip"))
			case "secret":
				cfg["proofKey"] = "invalid"
			}
			m := &mocks{resources: map[string]pulumi.MockResourceArgs{}}
			err := pulumi.RunErr(func(ctx *pulumi.Context) error { _, e := Deploy(ctx, cfg, opts); return e }, pulumi.WithMocks("test", "dev", m))
			if err == nil {
				t.Fatal("invalid plan accepted")
			}
			for _, r := range m.resources {
				if strings.HasPrefix(r.TypeToken, "aws:") {
					t.Fatal("preflight registered", r.Name)
				}
			}
		})
	}
}
func TestDeploymentSeparatesProofsGrantsAndScopesSecrets(t *testing.T) {
	cfg, opts := fixtures(t)
	m := &mocks{resources: map[string]pulumi.MockResourceArgs{}}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		d, e := Deploy(ctx, cfg, opts)
		if e == nil && len(d.Functions) != 3 {
			t.Fatal("missing handlers")
		}
		return e
	}, pulumi.WithMocks("test", "dev", m))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range m.resources {
		if r.TypeToken == "aws:cognito/userPool:UserPool" || r.TypeToken == "aws:apigatewayv2/api:Api" {
			t.Fatal("verification deployed identity or host API")
		}
		if r.TypeToken != "aws:lambda/function:Function" {
			continue
		}
		count++
		env := r.Inputs["environment"].ObjectValue()["variables"].ObjectValue()
		if env["TABLE_NAME"].StringValue() != "verification-proofs" || env["GRANT_TABLE"].StringValue() != "provided-grants_table_name" {
			t.Fatal("state stores mixed", env)
		}
		if _, ok := env["PROOF_KEY"]; ok {
			t.Fatal("plaintext proof key")
		}
		if env["PROOF_KEY_SECRET_VERSION"].StringValue() != "version-1" {
			t.Fatal("unpinned secret")
		}
	}
	if count != 3 {
		t.Fatal(count)
	}
	for _, n := range []string{"signup", "resend", "confirm"} {
		p := m.resources["verifyemail-"+n+"-permissions"].Inputs["policy"].StringValue()
		if !strings.Contains(p, "arn:email-proof-key") || strings.Contains(p, "secretsmanager:*") {
			t.Fatal("secret scope", p)
		}
		if n != "confirm" && strings.Contains(p, "provided-grants_table_arn") {
			t.Fatal("unneeded grant writer", p)
		}
	}
	if !m.resources["verification-proofs"].Inputs["serverSideEncryption"].ObjectValue()["enabled"].BoolValue() {
		t.Fatal("unencrypted proofs")
	}
}

// Existing Attestra stacks use route-<endpoint>. Keep their physical routes
// when adopting the feature's names, including after a partially failed update.
func TestRoutesAliasExistingAttestraIdentities(t *testing.T) {
	cfg, opts := fixtures(t)
	m := &mocks{resources: map[string]pulumi.MockResourceArgs{}}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error { _, e := Deploy(ctx, cfg, opts); return e }, pulumi.WithMocks("attestra-application", "dev", m))
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"signup", "resend", "confirm"} {
		r, ok := m.resources["verifyemail-"+endpoint+"-route"]
		if !ok {
			t.Fatal("missing route", endpoint)
		}
		want := "urn:pulumi:dev::attestra-application::aws:apigatewayv2/route:Route::route-" + endpoint
		found := false
		for _, urn := range r.RegisterRPC.GetAliasURNs() {
			if urn == want {
				found = true
			}
		}
		for _, alias := range r.RegisterRPC.GetAliases() {
			if alias.GetUrn() == want || (alias.GetSpec() != nil && alias.GetSpec().GetName() == "route-"+endpoint) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s lacks migration alias %s", r.Name, want)
		}
		if r.Inputs["routeKey"].StringValue() != "POST /"+endpoint {
			t.Errorf("endpoint changed: %s", endpoint)
		}
	}
}
