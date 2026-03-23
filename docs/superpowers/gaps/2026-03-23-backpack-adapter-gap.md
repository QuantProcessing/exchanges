# Backpack Adapter Gap

## Keep

- Market-cache-driven metadata loading and focused request/mapping coverage.
- Localized handling of Backpack-specific client-id constraints.
- Clear exchange-specific mapping seams where the adapter already translates unified behavior into Backpack wire formats.

## Change

- Classify Backpack explicitly as a REST-only adapter for order transport in this first pass.
  Acceptance: constructors set REST mode explicitly, code comments describe the REST-only contract, and deterministic tests assert the REST-only default.
- Converge Backpack SDK naming toward repository-standard request verbs using staged compatibility shims where needed.
  Acceptance: adapter call sites use preferred wrapper names for order placement and orderbook reads, while deferred WS naming remains documented as out of scope for this pass.
- Clean up stable free-form failures so missing-resource and unsupported paths use shared sentinel errors instead.
  Acceptance: stable unsupported paths return `ErrNotSupported`, stable missing-resource paths use `ErrSymbolNotFound` or the correct shared sentinel, and targeted tests cover those behaviors.
- Add stronger SDK-level tests around constructor seams, private/public REST behavior, and REST-only transport expectations.
  Acceptance: deterministic tests cover the new constructor seams and the compatibility wrapper behavior added in this pass.

## Defer

- Any future move from REST-only transport to full `OrderMode` switching.
- Repository-wide decisions about whether stream logic should default back into the main adapter files.
- Broad repository-wide WS naming normalization beyond the Backpack-specific convergence work in this pass.
- `WSClient` naming stays deferred until the repository resolves the open WS naming decision in the adapter-layering spec.
