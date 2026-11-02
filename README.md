# go.apexfission.aws.verifyemail

Independent email verification feature extracted from go.apexfission.aws.auth. The reusable state machine, API handlers, DynamoDB proof store, SES sender, Secrets Manager reader and optional Cognito enrollment adapter live here. No authentication or application module is imported.

The existing protocol is preserved: local token A plus email-link token B, or B plus the six-digit manual code C; ten-minute proofs, resend invalidation/cooldown, budgets, conditional claims, single-use confirmation and sign-in recovery after interrupted account creation. Generic Go hosts inject `email.Store`, `email.Sender` and `email.Identity` into `email.Service`; the identity interface determines eligibility, completion and optional session issuance.

## Native feature contract

`feature.Declare("email-verification")` requires `identity.email-enrollment/v1`. This implemented adapter takes a Cognito pool/client, the authentication grant table, a host HTTP API, SES identity and account/region. It does not create those resources and never calls an authentication-specific RPC.

The optional `infrastructure` module provisions its own encrypted/PITR/TTL proof table, pinned Secrets Manager key version, three Linux ARM64 Lambdas, scoped IAM and POST /signup, /resend and /confirm on the supplied API. Only confirmation can write authentication grants, restricted to `grant#*` keys. The generic state machine can be embedded with other identity implementations; this native deployment adapter specifically supports Cognito.

Proof transactions use `TABLE_NAME`; authentication grants use the required, distinct `GRANT_TABLE`. No fallback silently combines them. A caller that previously stored both in one table must plan a migration or drain old requests. The application owns the panel, configuration source choices, API stage/domain, and reset coordination. Declarations perform no AWS operations.

## Configuration and secrets

`feature.ConfigurationContract` declares application settings (link origin, diagnostics) and artifact settings (sender, proof-key delivery). The key is supplied as secret Pulumi configuration, written to Secrets Manager and retrieved by exact version once at cold start. Lambda environments contain ARN/version references, never the raw key. This is pinned-version delivery, not automatic rotation; changing a key needs a rollout that preserves pending proofs. Diagnostics affect public error behavior and require restricted access.

## Validate and build

Go 1.27.3+:

```sh
go test -race ./...
go vet ./...
go run ./cmd/build-lambdas -out ./dist
cd infrastructure
go test -race ./...
go vet ./...
```

The root module works without the Pulumi SDK. The optional infrastructure module uses a same-repository replacement for development; consumers resolve its pinned root dependency normally. See [infrastructure](infrastructure/README.md) and the composing application's [two-feature guide](https://github.com/lambdawalker/go.attestra.aws/blob/feat/two-feature-composition/docs/two-feature-composition.md). No AWS deployment, key rotation or data migration has been performed.
