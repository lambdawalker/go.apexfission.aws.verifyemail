package feature

import (
	"errors"
	composer "github.com/lambdawalker/go.infra.composer"
	"testing"
)

func TestMissingIdentityFailsBeforeExecution(t *testing.T) {
	_, err := composer.NewApplication("test", Declare("verifyemail")).Plan(composer.Environment{Name: "dev"})
	var e *composer.ValidationError
	if !errors.As(err, &e) || e.Diagnostics[0].Code != "IC_MISSING_PROVIDER" {
		t.Fatalf("missing integration accepted: %v", err)
	}
}
func TestInjectedIdentityContractDrivesAllInputs(t *testing.T) {
	values := map[string]composer.Value{}
	for k := range IntegrationContract().Fields {
		values[k] = composer.Literal("provided-" + k)
	}
	provider := &composer.Feature{ID: "custom-identity", Offerings: []composer.Offering{{ID: "enrollment", Contract: IntegrationContract(), Values: values}}}
	app := composer.NewApplication("test", provider, Declare("verifyemail"))
	if err := app.Inject("verifyemail", IntegrationRequirement, provider.ID, "enrollment"); err != nil {
		t.Fatal(err)
	}
	p, err := app.Plan(composer.Environment{Name: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Resources()) != 1 || len(p.Resources()[0].Inputs) != 10 {
		t.Fatal("lost integration fields", p)
	}
	if len(ConfigurationContract("verifyemail", "dev").Properties) != 4 {
		t.Fatal("incomplete configuration")
	}
}
