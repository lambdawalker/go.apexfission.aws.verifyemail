// Package feature declares email verification without provider side effects.
package feature

import (
	composer "github.com/lambdawalker/go.infra.composer"
	"github.com/lambdawalker/go.infra.composer/configuration"
)

const Kind = "apexfission:verifyemail:Backend"
const IntegrationRequirement = "identity_integration"

// IntegrationContract describes the supplied Cognito enrollment adapter. Other Go
// hosts can implement email.Identity without using this native deployment adapter.
func IntegrationContract() composer.Contract {
	fields := map[string]composer.Field{}
	for _, k := range []string{"pool_id", "pool_arn", "client_id", "grants_table_name", "grants_table_arn", "api_id", "api_execution_arn", "sender_identity_arn", "region", "account_id"} {
		fields[k] = composer.Field{Type: composer.String}
	}
	return composer.Contract{ID: "identity.email-enrollment", Major: 1, Fields: fields}
}
func Declare(id string) *composer.Feature {
	inputs := map[string]composer.Value{}
	for k := range IntegrationContract().Fields {
		inputs[k] = composer.RequirementRef(id, IntegrationRequirement, k)
	}
	return &composer.Feature{ID: id, Source: composer.Origin(), Requirements: []composer.Requirement{{ID: IntegrationRequirement, Contract: IntegrationContract(), Source: composer.Origin()}}, Resources: []composer.Resource{{ID: "backend", Kind: Kind, Source: composer.Origin(), Inputs: inputs, Outputs: map[string]composer.Field{"proof_table_name": {Type: composer.String}, "proof_table_arn": {Type: composer.String}}}}, Offerings: []composer.Offering{{ID: "verification", Contract: composer.Contract{ID: "email.verification", Major: 1, Fields: map[string]composer.Field{"proof_table_name": {Type: composer.String}}}, Values: map[string]composer.Value{"proof_table_name": composer.ResourceRef(id+".backend", "proof_table_name")}}}}
}

type Configurable struct {
	Name, NativeKey, Risk, Reader string
	Type                          composer.Type
	Category                      configuration.Category
	Sensitivity                   configuration.Sensitivity
}

func ConfigurableProperties() []Configurable {
	return []Configurable{
		{"app_origin", "APP_ORIGIN", "verification-link-origin", "env", composer.String, configuration.Application, configuration.Public},
		{"sender_address", "SENDER_ADDRESS", "sender-iam-rollout", "env", composer.String, configuration.Artifact, configuration.Public},
		{"diagnostic_mode", "DIAGNOSTIC_MODE", "public-error-disclosure", "env", composer.Boolean, configuration.Application, configuration.Public},
		{"proof_key", "SecretString", "cryptographic-key-migration", "secretsmanager-cold-start", composer.String, configuration.Artifact, configuration.Confidential},
	}
}
func ConfigurationContract(id, environment string) configuration.Declaration {
	d := configuration.Declaration{Environment: environment, PolicyVersion: "verifyemail-configuration-v1"}
	for _, p := range ConfigurableProperties() {
		controls := []string{"scoped-access"}
		if p.Sensitivity == configuration.Confidential {
			controls = append(controls, "encrypted")
		}
		d.Properties = append(d.Properties, configuration.Property{Name: p.Name, Type: p.Type, Category: p.Category, Sensitivity: p.Sensitivity, Required: true, RequiredControls: controls})
		d.Consumers = append(d.Consumers, configuration.Consumer{Artifact: id + ".backend", Property: p.Name, Readers: []string{p.Reader}})
	}
	return d
}
