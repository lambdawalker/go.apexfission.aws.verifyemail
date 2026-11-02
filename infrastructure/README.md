# Email verification native adapter

Call `Validate(cfg, Options{ArtifactDir, Environment, Integration})` before registering any resources. The integration map has exactly the ten string inputs declared by `feature.IntegrationContract`: pool_id, pool_arn, client_id, grants_table_name, grants_table_arn, api_id, api_execution_arn, sender_identity_arn, region, account_id. Inputs may remain native Pulumi outputs. `Deploy` repeats validation and returns the proof table and three functions for application reset coordination.

Configuration: appOrigin (HTTPS origin), senderDomain, senderAddress, diagnosticMode (optional boolean), proofKey (base64 key of at least 32 bytes supplied as secret configuration), emailSubject (optional branding). The three archives signup.zip, resend.zip and confirm.zip must exist. The application maps its settings to `feature.ConfigurationContract`; emailSubject is a deployment-only optional branding override rather than a live administration control.

`PreflightOptions` creates placeholders for integration fields already checked by composer, solely to validate config/archive availability before native outputs exist. Actual deployment always receives the resolved native inputs. This adapter creates neither a Cognito pool nor an HTTP API; standalone hosting can supply those from any conforming provider. One verification instance per stack is supported. A generic Go embedding can use other identity providers through email.Identity without this adapter.

The existing SES identity is shared with Cognito OTP mail. Readiness/production SES access is an operator requirement; local mocks cannot verify it. Reset coordination must pause all three functions before clearing proof and grant tables, include their exact resources in reset IAM, then resume them. See the application's implementation and tests.

## Ownership labels

Native resources receive the seven canonical `ic:` tags where their provider schema supports tags, with safe description prefixes on mutable fields. No physical identifiers change. `resourceInventory` lists every registered AWS resource, including unsupported tagging cases and child identifiers. The application supplies per-feature scopes; standalone adapters derive validated project/environment/origin defaults. Use `ownershipDomain` in the application stack to override the hostname label; shared control-plane resources use environment `shared`. Legacy SES adapter resources remain explicitly inventory-only.
