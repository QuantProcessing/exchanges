# Adapter Capability Policy

Adapter capabilities are claims, not aspirations. A capability may be marked
`Yes` only when the adapter implements the matching interface and passes shared
contract tests. A missing capability must be represented as unsupported
behavior or `Planned`, not as a no-op success path.

## Capability Matrix

Use `docs/parity/adapter-capability-matrix.md` as the current support matrix.
It separates:

- data snapshots;
- data streams;
- account snapshots;
- submit, cancel, modify, and query;
- order, fill, and position reports;
- private stream;
- resubscribe;
- mass status;
- order lists.

Use `docs/parity/adapter-live-test-policy.md` as the live-test gate policy for
public reads, private reads, and mutating live write tests.

Private stream and resubscribe are separate lifecycle claims. An adapter may
stream private execution events without proving restart resubscription, or may
support resubscription without claiming mass-status recovery.

## Contract Rule

`testsuite.AdapterCapabilitySuite` enforces the mapping between
`venue.DeclaredCapabilities` and optional interfaces such as
`venue.ExecutionResubscriber`, `venue.ExecutionMassStatusGenerator`, and
`venue.OrderListSubmitter`.

Run:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```

## Live Test Gates

Read-only SDK and adapter tests may run by default when they do not mutate
exchange state. Any live write test must be gated by both credentials and an
exchange-specific enable flag, such as `BYBIT_ENABLE_WS_ORDER_TESTS=1`.

The phrase live write means a test can place, cancel, modify, transfer, or
otherwise mutate venue state. Such tests must skip clearly unless the operator
opted in with the documented flag and credentials.

## Release Rule

Release notes must list:

- every adapter currently counted as supported;
- every planned capability still missing;
- extension targets still outside the current SDK scope;
- the exact contract command used as evidence.
