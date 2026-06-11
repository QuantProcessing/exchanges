# Official API Parity Rules

Every official spot/perp REST endpoint, WebSocket channel, and WebSocket API
operation must be represented in an exchange-specific parity file.

Allowed statuses:

- `implemented-sdk`: a typed low-level SDK method exists.
- `implemented-adapter`: a typed SDK method exists and is exposed through an adapter or extension interface.
- `implemented-raw`: a signed/routed raw SDK method exists because the official schema is intentionally broad or unstable.
- `missing-sdk`: the official endpoint is in scope and no typed or raw SDK method exists yet.
- `missing-adapter`: the SDK method exists, but an existing adapter interface should expose it and does not yet.
- `intentionally-unsupported`: the endpoint is official but outside this repository's spot/perp trading scope.
- `blocked-by-official-api`: the official documentation is incomplete, contradictory, gated, or unavailable enough that implementation would be unsafe.
- `deprecated-official`: the official docs mark the endpoint as deprecated.

For `implemented-*` rows, `Local Symbol` must name the local Go method or interface that owns the implementation.

The alignment project is complete only when there are zero `missing-sdk` and zero `missing-adapter` rows for in-scope spot/perp APIs.

Accessed date for the initial source pass: 2026-06-10.
