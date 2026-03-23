# Exchange Adapter Review Checklist

Use this checklist for every new adapter and every major adapter refactor.

Definitions and policy details live in [2026-03-23-exchange-adapter-layering-design.md](/Users/dylan/Code/exchanges/docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md). This checklist is a review companion, not a replacement for the spec.

- [ ] Constructor semantics are explicit and stable. Confirm constructor names, auth policy, metadata-loading behavior, and any fail-fast or permissive behavior are documented and consistent.
- [ ] Base-symbol semantics are preserved at the adapter boundary, or any compatibility exception is explicit. Confirm unified methods take base symbols unless a documented compatibility escape hatch exists.
- [ ] Shared sentinel errors are used consistently. Confirm stable unsupported, auth, order-not-found, and symbol-not-found paths use shared sentinels and still allow `errors.Is`.
- [ ] `OrderMode` classification is explicit and accurate. Confirm the adapter is clearly documented and tested as full switching, REST-only, or approved hybrid as defined in the spec.
- [ ] Adapter, helper, and SDK boundaries are clear. Confirm main adapters orchestrate behavior while helpers and SDK files keep one stable concern each.
- [ ] Exchange-specific complexity is concentrated into a small number of files. Confirm local exceptions stay localized instead of creating a second hidden architecture.
- [ ] The main adapter files remain the primary reading entrypoints. Confirm a maintainer can open the market adapter file and understand capabilities, unified methods, and helper ownership.
- [ ] SDK naming aligns with repository conventions. Confirm request and stream helpers use repository-standard verbs unless a documented protocol constraint requires a local deviation.
- [ ] The adapter satisfies the minimum test matrix. Confirm each concrete market adapter has compliance, order, order-query, lifecycle, and local-state coverage, plus exchange-specific tests for unique behavior.
- [ ] Deviations from this standard are explicitly justified. Confirm every structural or behavioral exception is documented, small in scope, and visible in review.
