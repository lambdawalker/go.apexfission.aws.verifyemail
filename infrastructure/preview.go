package infrastructure

import (
	"encoding/base64"
	"github.com/lambdawalker/go.apexfission.aws.verifyemail/artifacts"
	"github.com/lambdawalker/go.infra.composer/adapters/pulumi/reportpreview"
	"github.com/lambdawalker/go.infra.composer/reporting"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// PreviewFeature is the canonical feature selector used by consuming application reports.
// It does not alter the native deployment ownership defaults.
const PreviewFeature = "email-verification"

// Preview constructs the actual native feature offline. External host bindings
// and private empty archives are fixtures; no provider or deployment is invoked.
func Preview(config reportpreview.Config, environment string) (out []reporting.Artifact, err error) {
	cfg := reportpreview.Defaults(config)
	names := []string{}
	for _, a := range artifacts.Lambdas() {
		names = append(names, a.Name)
	}
	err = reportpreview.Archives(names, func(dir string) error {
		var e error
		out, e = reportpreview.Catalog(environment, PreviewFeature, "go.apexfission.aws.verifyemail", func(ctx *pulumi.Context) error {
			cfg["proofKey"] = base64.StdEncoding.EncodeToString(make([]byte, 32))
			integration := map[string]pulumi.StringInput{}
			for _, k := range integrationFields() {
				integration[k] = pulumi.String("external-" + k)
			}
			_, err := Deploy(ctx, cfg, Options{ArtifactDir: dir, Environment: environment, Integration: integration})
			return err
		})
		return e
	})
	return
}
