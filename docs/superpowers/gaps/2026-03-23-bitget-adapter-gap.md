# Bitget Adapter Gap

## Keep

- Existing request, orderbook, and account-mode test depth.
- The classic/private-profile split where it reflects durable exchange account-model constraints.
- The current adapter shape where exchange-specific transport and account handling stay localized instead of leaking across the package.

## Change

- Classify Bitget explicitly as a controlled hybrid transport adapter and document the REST-default and WS-switchable order paths.
  Status: landed in the initial rollout.
  Acceptance: code comments name the switched subset, existing WS-order-mode tests remain green, and the package docs describe the adapter as controlled hybrid rather than implicitly drifting into that shape.
- Normalize funding code placement so funding behavior lives in the package location intended by the standard instead of remaining scattered.
  Status: landed in the initial rollout.
  Acceptance: perp funding methods move into `bitget/funding.go`, and explicit unsupported behavior is covered by deterministic tests.
- Document the private-profile boundary as a controlled package exception rather than a default repository pattern.
  Status: landed in the initial rollout.
  Acceptance: `private_profile.go` and related entrypoints explain why the split exists and why it is Bitget-specific.
- Add tests that explicitly cover the hybrid contract, including the supported switched subset and the REST-default path.
  Status: landed in the initial rollout.
  Acceptance: the existing transport tests plus any added funding/contract tests cover REST default, documented WS-switched operations, and no silent fallback outside the supported subset.

## Defer

- Deferred: repository-wide resolution of whether hybrid transport is a temporary convergence state or a permanent approved shape.
- Deferred: broader file-layout changes beyond funding placement and private-profile boundary cleanup.
- Deferred: repository-wide WS naming or constructor-default decisions not required to document the Bitget contract in this pass.
