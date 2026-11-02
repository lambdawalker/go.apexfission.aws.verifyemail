# Email verification ownership

This feature must not import go.apexfission.aws.auth or go.attestra.aws. Keep email.Identity injectable; the optional native adapter consumes the identity.email-enrollment/v1 contract. Proof transactions and authentication grants use distinct tables. Preserve single-use proof, retry, limit, expiry and account recovery behavior. Run Go tests/race/vet in root and infrastructure. Do not deploy AWS, migrate state, seed real data or rotate proof keys as validation. Do not commit generated archives, stack config or credentials.
