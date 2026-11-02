// Package infrastructure deploys email verification onto a supplied HTTP API
// using the declared Cognito enrollment integration. It never provisions Cognito.
package infrastructure

import "github.com/lambdawalker/go.infra.composer/adapters/pulumi/awslabels"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lambdawalker/go.apexfission.aws.verifyemail/artifacts"
	"github.com/lambdawalker/go.apexfission.aws.verifyemail/feature"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/secretsmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Config interface {
	Get(string) string
	Require(string) string
	RequireSecret(string) pulumi.StringOutput
}
type Options struct {
	ArtifactDir, Environment string
	Integration              map[string]pulumi.StringInput
}
type Deployment struct {
	Table     *dynamodb.Table
	Functions []*lambda.Function
}

func integrationFields() []string {
	// Ordered so diagnostics and native serialization are reproducible.
	return []string{"pool_id", "pool_arn", "client_id", "grants_table_name", "grants_table_arn", "api_id", "api_execution_arn", "sender_identity_arn", "region", "account_id"}
}

// PreflightOptions supplies placeholders only for fields already validated by the
// composer contract. No placeholder is used to register a native resource.
func PreflightOptions(dir, environment string) Options {
	in := map[string]pulumi.StringInput{}
	for _, k := range integrationFields() {
		in[k] = pulumi.String("composition-preflight")
	}
	return Options{ArtifactDir: dir, Environment: environment, Integration: in}
}
func Validate(cfg Config, opts Options) error {
	if cfg == nil || strings.TrimSpace(opts.Environment) == "" {
		return fmt.Errorf("configuration and environment are required")
	}
	for _, k := range integrationFields() {
		v := opts.Integration[k]
		if v == nil {
			return fmt.Errorf("missing verification integration %s", k)
		}
		if s, ok := v.(pulumi.String); ok && strings.TrimSpace(string(s)) == "" {
			return fmt.Errorf("empty verification integration %s", k)
		}
	}
	for k := range opts.Integration {
		if _, ok := feature.IntegrationContract().Fields[k]; !ok {
			return fmt.Errorf("unknown verification integration %s", k)
		}
	}
	origin, e := url.Parse(cfg.Get("appOrigin"))
	if e != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return fmt.Errorf("appOrigin must be a credential-free HTTPS origin")
	}
	sender := cfg.Get("senderAddress")
	address, e := mail.ParseAddress(sender)
	if e != nil || address.Address != sender || !strings.HasSuffix(sender, "@"+cfg.Get("senderDomain")) || cfg.Get("senderDomain") == "" {
		return fmt.Errorf("senderAddress must belong to senderDomain")
	}
	key, e := base64.StdEncoding.DecodeString(cfg.Get("proofKey"))
	if e != nil || len(key) < 32 {
		return fmt.Errorf("proofKey must be base64 of at least 32 random bytes supplied through secret configuration")
	}
	if value := cfg.Get("diagnosticMode"); value != "" {
		if _, e := strconv.ParseBool(value); e != nil {
			return fmt.Errorf("diagnosticMode must be boolean")
		}
	}
	if opts.ArtifactDir == "" {
		return fmt.Errorf("ArtifactDir is required")
	}
	for _, a := range artifacts.Lambdas() {
		p := filepath.Join(opts.ArtifactDir, a.Name+".zip")
		info, e := os.Stat(p)
		if e != nil {
			return fmt.Errorf("build verification archive %s: %w", p, e)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("verification archive must be a regular file: %s", p)
		}
	}
	return nil
}
func Deploy(ctx *pulumi.Context, cfg Config, opts Options) (*Deployment, error) {
	if e := Validate(cfg, opts); e != nil {
		return nil, e
	}
	ctx, labels, labelErr := awslabels.Ensure(ctx, opts.Environment, cfg.Get("appOrigin"), "verifyemail")
	if labelErr != nil {
		return nil, labelErr
	}
	in := opts.Integration
	table, e := dynamodb.NewTable(ctx, "verification-proofs", &dynamodb.TableArgs{BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String("id"), Attributes: dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String("id"), Type: pulumi.String("S")}}, Ttl: &dynamodb.TableTtlArgs{AttributeName: pulumi.String("ttl"), Enabled: pulumi.Bool(true)}, ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}}, awslabels.Option(ctx))
	if e != nil {
		return nil, e
	}
	secret, e := secretsmanager.NewSecret(ctx, "email-proof-key", &secretsmanager.SecretArgs{Description: pulumi.String("Email verification HMAC key; immutable version delivery")}, awslabels.Option(ctx))
	if e != nil {
		return nil, e
	}
	version, e := secretsmanager.NewSecretVersion(ctx, "email-proof-key-version", &secretsmanager.SecretVersionArgs{SecretId: secret.ID(), SecretString: pulumi.ToSecret(cfg.RequireSecret("proofKey")).(pulumi.StringOutput)}, awslabels.Option(ctx))
	if e != nil {
		return nil, e
	}
	out := &Deployment{Table: table}
	for _, artifact := range artifacts.Lambdas() {
		name := artifact.Name
		role, e := iam.NewRole(ctx, "verifyemail-"+name+"-role", &iam.RoleArgs{AssumeRolePolicy: pulumi.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`)}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = iam.NewRolePolicyAttachment(ctx, "verifyemail-"+name+"-logs", &iam.RolePolicyAttachmentArgs{Role: role.Name, PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		policy := pulumi.All(table.Arn, secret.Arn, in["pool_arn"], in["grants_table_arn"], in["sender_identity_arn"], in["region"], in["account_id"]).ApplyT(func(v []any) (string, error) {
			statement := func(actions, resource any) map[string]any {
				return map[string]any{"Effect": "Allow", "Action": actions, "Resource": resource}
			}
			actions := []string{"dynamodb:PutItem", "dynamodb:UpdateItem"}
			if name != "signup" {
				actions = append(actions, "dynamodb:GetItem")
			}
			statements := []any{statement(actions, v[0]), statement("secretsmanager:GetSecretValue", v[1]), statement("cognito-idp:AdminGetUser", v[2])}
			if name == "confirm" {
				statements = append(statements, statement([]string{"cognito-idp:AdminCreateUser", "cognito-idp:AdminInitiateAuth", "cognito-idp:AdminRespondToAuthChallenge"}, v[2]))
				grant := statement("dynamodb:PutItem", v[3])
				grant["Condition"] = map[string]any{"ForAllValues:StringLike": map[string]any{"dynamodb:LeadingKeys": []string{"grant#*"}}}
				statements = append(statements, grant)
			} else {
				// SES sandbox checks the recipient identity too; scope both to this account
				// and region, and always constrain the From address.
				send := statement("ses:SendEmail", []any{v[4], fmt.Sprintf("arn:aws:ses:%s:%s:identity/*", v[5], v[6])})
				send["Condition"] = map[string]any{"StringEquals": map[string]any{"ses:FromAddress": cfg.Get("senderAddress")}}
				statements = append(statements, send)
			}
			b, e := json.Marshal(map[string]any{"Version": "2012-10-17", "Statement": statements})
			return string(b), e
		}).(pulumi.StringOutput)
		grant, e := iam.NewRolePolicy(ctx, "verifyemail-"+name+"-permissions", &iam.RolePolicyArgs{Role: role.ID(), Policy: policy}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		archive, e := filepath.Abs(filepath.Join(opts.ArtifactDir, name+".zip"))
		if e != nil {
			return nil, e
		}
		vars := pulumi.StringMap{"TABLE_NAME": table.Name, "GRANT_TABLE": in["grants_table_name"], "POOL_ID": in["pool_id"], "CLIENT_ID": in["client_id"], "APP_ORIGIN": pulumi.String(cfg.Get("appOrigin")), "SENDER_ADDRESS": pulumi.String(cfg.Get("senderAddress")), "PROOF_KEY_SECRET_ARN": secret.Arn, "PROOF_KEY_SECRET_VERSION": version.VersionId, "DIAGNOSTIC_MODE": pulumi.String(strconv.FormatBool(cfg.Get("diagnosticMode") == "true")), "EMAIL_SUBJECT": pulumi.String(cfg.Get("emailSubject"))}
		fn, e := lambda.NewFunction(ctx, "email-"+name, &lambda.FunctionArgs{Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap"), Architectures: pulumi.StringArray{pulumi.String("arm64")}, Role: role.Arn, Code: pulumi.NewFileArchive(archive), Timeout: pulumi.Int(25), MemorySize: pulumi.Int(256), Environment: &lambda.FunctionEnvironmentArgs{Variables: vars}}, pulumi.DependsOn([]pulumi.Resource{grant}), awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		out.Functions = append(out.Functions, fn)
		integration, e := apigatewayv2.NewIntegration(ctx, "verifyemail-"+name+"-integration", &apigatewayv2.IntegrationArgs{ApiId: in["api_id"], IntegrationType: pulumi.String("AWS_PROXY"), IntegrationUri: fn.InvokeArn, PayloadFormatVersion: pulumi.String("2.0")}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = apigatewayv2.NewRoute(ctx, "verifyemail-"+name+"-route", &apigatewayv2.RouteArgs{ApiId: in["api_id"], RouteKey: pulumi.String("POST /" + name), Target: pulumi.Sprintf("integrations/%s", integration.ID())}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = lambda.NewPermission(ctx, "verifyemail-allow-"+name, &lambda.PermissionArgs{Action: pulumi.String("lambda:InvokeFunction"), Function: fn.Name, Principal: pulumi.String("apigateway.amazonaws.com"), SourceArn: pulumi.Sprintf("%s/*/POST/%s", in["api_execution_arn"], name)}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
	}
	if labels != nil {
		if err := labels.Export(ctx, "resourceInventory"); err != nil {
			return nil, err
		}
	}
	return out, nil
}
