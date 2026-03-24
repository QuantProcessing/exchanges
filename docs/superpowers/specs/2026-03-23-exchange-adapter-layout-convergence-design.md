# Exchange Adapter Layout Convergence Design

## Goal

Converge the remaining package-level file layout outliers without changing adapter behavior.

This phase is intentionally narrower than the earlier layering and naming rollouts. It only targets packages where the current file split still makes the package harder to read than necessary.

## Scope

This rollout only touches:

- `backpack`
- `aster`
- `binance`

It does not change `bitget` layout. `private_profile.go` and `order_request.go` remain accepted controlled exceptions under the existing layering spec.

## Decisions

### 1. Backpack stream layout

`backpack/perp_streams.go` and `backpack/spot_streams.go` should be replaced by a single `streams.go`.

Reasoning:

- the current split creates two secondary entrypoints for one concern
- the stream logic is still the same category of responsibility for both markets
- one shared stream file is consistent with the repository rule that auxiliary files should isolate one stable concern

This rollout may also move small stream-only helpers if doing so makes the resulting `streams.go` self-contained.

### 2. Aster orderbook layout

`aster/perp_orderbook.go` and `aster/spot_orderbook.go` should converge to `aster/orderbook.go`.

Reasoning:

- both files implement the same kind of local orderbook synchronization model
- the current split mostly duplicates structure rather than isolating meaningfully different behavior
- a single file is closer to the repository default for packages that share one synchronization model

The implementation may keep separate perp and spot orderbook types inside the unified file. The goal is file-level convergence, not forced type unification.

### 3. Binance orderbook layout

`binance/perp_orderbook.go` and `binance/spot_orderbook.go` should converge to `binance/orderbook.go`.

The same rule and rationale as Aster apply here.

## Non-Goals

This rollout does not:

- change adapter or SDK behavior
- rename orderbook types
- force type-level deduplication across spot and perp
- move `bitget` away from its accepted private-profile layout
- revisit SDK naming, transport policy, or constructor policy

## Acceptance Criteria

The rollout is complete when:

- `backpack` has a single `streams.go`
- `aster` has a single `orderbook.go`
- `binance` has a single `orderbook.go`
- package behavior is unchanged
- `go test -short ./...` passes
