# Adding Exchange Adapters Skill Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the project-local `adding-exchange-adapters` skill so it can reliably guide complex new exchange adapter work in this repository.

**Architecture:** Keep `SKILL.md` as the short routing and done-criteria entrypoint, and move detailed guidance into four focused reference files under `references/`. Validate the result with documentation-focused TDD: baseline pressure scenario, skill rewrite, then post-change pressure verification.

**Tech Stack:** Markdown documentation, existing project skills, subagent pressure testing, git worktree workflow

---

### Task 1: Capture Baseline Skill Failure Modes

**Files:**
- Read: `.claude/skills/adding-exchange-adapters/SKILL.md`
- Read: `/home/xiguajun/.agents/skills/exchanges/SKILL.md`
- Read: `docs/superpowers/specs/2026-03-22-adding-exchange-adapters-skill-design.md`
- Read: `backpack/order_request.go`
- Read: `backpack/spot_streams.go`
- Read: `backpack/adapter_test.go`
- Create: `docs/superpowers/plans/2026-03-22-adding-exchange-adapters-skill.md`

- [ ] **Step 1: Write the baseline pressure-test prompt**

Create a prompt that asks a fresh subagent how to add a complex adapter with:

- dedicated `sdk/` decision
- `FetchOrder` versus `FetchOpenOrders` semantics
- private WebSocket and local-state support expectations
- live `testsuite` wiring requirements

Expected outcome: the current skill should leave important decisions ambiguous or under-specified.

- [ ] **Step 2: Run baseline pressure test against the current skill state**

Run: dispatch one fresh reviewer/pressure subagent with only the current skill paths and the prompt from Step 1

Expected: the subagent identifies missing guidance or makes incomplete recommendations, demonstrating why the skill needs expansion.

- [ ] **Step 3: Summarize the observed gaps**

Record the concrete baseline failures to drive the documentation edits:

- missing SDK boundary decision rules
- weak order-query semantics guidance
- unclear private-stream/local-state readiness
- missing live-test wiring conventions

- [ ] **Step 4: Cross-check the baseline gaps against Backpack lessons**

Review the Backpack implementation files listed above and confirm the baseline failures map to real work this repository already had to do:

- `backpack/order_request.go` for order-contract and `ClientID` lessons
- `backpack/spot_streams.go` for explicit `exchanges.ErrNotSupported` behavior
- `backpack/adapter_test.go` for resilient `.env` lookup and shared `testsuite` wiring

Expected: the gaps are grounded in real repository history, not only hypothetical pressure-test failures.

- [ ] **Step 5: Commit the planning checkpoint**

```bash
git add docs/superpowers/plans/2026-03-22-adding-exchange-adapters-skill.md
git commit -m "docs: add adapter skill implementation plan"
```

### Task 2: Rewrite The Main Skill Entry Point

**Files:**
- Modify: `.claude/skills/adding-exchange-adapters/SKILL.md`
- Read: `docs/superpowers/specs/2026-03-22-adding-exchange-adapters-skill-design.md`
- Read: `/home/xiguajun/.agents/skills/exchanges/SKILL.md`

- [ ] **Step 1: Write the failing documentation expectation**

Before editing, define the exact additions the main skill must provide:

- adapter-specific `Before You Write Code`
- `Architecture Decisions`
- `Order Contract Checklist`
- `Private API Readiness`
- `Live Test Readiness`
- `Do Not Ship If`

Expected failure: the current `SKILL.md` does not yet cover these sections at the required depth.

- [ ] **Step 2: Edit `SKILL.md` with the minimal new routing structure**

Add concise sections that:

- explicitly complement `exchanges` instead of duplicating it
- require the agent to choose the closest peer package by market coverage and auth model before writing code
- classify adapter targets up front as public-data-only, trading-capable, lifecycle-capable, or local-state-capable
- route exact `testsuite` expectations from those capability levels into `references/`
- define concrete `sdk/` decision criteria and forbidden adapter-layer responsibilities
- define the private-support matrix, including `WatchOrders` as a hard prerequisite for local state and `WatchPositions` as additive
- route complex details into `references/`
- define `Do Not Ship If` stop conditions as explicit fail gates
- require `exchanges.ErrNotSupported` for unsupported shared surfaces

- [ ] **Step 3: Verify the rewritten skill stays concise and repository-specific**

Run: `sed -n '1,260p' .claude/skills/adding-exchange-adapters/SKILL.md`

Expected: the file reads as an entrypoint and routing guide, not a full tutorial and not a duplicate of `exchanges`.

- [ ] **Step 4: Commit the main skill rewrite**

```bash
git add .claude/skills/adding-exchange-adapters/SKILL.md
git commit -m "docs: expand adapter skill entrypoint"
```

### Task 3: Add Focused Reference Documents

**Files:**
- Create: `.claude/skills/adding-exchange-adapters/references/sdk-boundaries.md`
- Create: `.claude/skills/adding-exchange-adapters/references/order-semantics.md`
- Create: `.claude/skills/adding-exchange-adapters/references/private-streams-and-localstate.md`
- Create: `.claude/skills/adding-exchange-adapters/references/live-test-wiring.md`
- Read: `exchange.go`
- Read: `errors.go`
- Read: `local_state.go`
- Read: `backpack/adapter_test.go`
- Read: representative peer SDK layouts such as `backpack/sdk/`, `binance/sdk/`, `nado/sdk/`, `grvt/sdk/`

- [ ] **Step 1: Write the failing content checklist for each reference**

Define the required content before writing:

- `sdk-boundaries.md`: choose nearest peer layout, not a canonical SDK tree; define adapter-versus-SDK responsibilities; define wire-type and mapping boundaries; cover when spot/perp should share SDK code versus split; include concrete anti-patterns
- `order-semantics.md`: current `FetchOrder` and `FetchOpenOrders` contracts only; include symbol filtering expectations, acceptable `FetchOrder` fallback strategies, and use of `exchanges.ErrOrderNotFound`
- `private-streams-and-localstate.md`: `WatchOrders` mandatory for LocalState, `WatchPositions` additive, `exchanges.ErrNotSupported` for unsupported streams
- `live-test-wiring.md`: Backpack-style `.env` lookup helper, `.env.example`, environment-variable naming, `adapter_test.go`, `testsuite` matrix, skip-flag guidance, stable symbol/quote selection, and the rule that live integration is incomplete without shared-suite coverage in `adapter_test.go`

Expected failure: none of these reference files exist yet.

- [ ] **Step 2: Add the four reference files**

Write each file as a concise decision manual:

- repository-specific
- concrete
- biased toward anti-pattern prevention
- free of generic exchange-integration background prose

- [ ] **Step 3: Verify cross-references and file paths**

Run:

```bash
rg -n "references/|ErrNotSupported|FetchOrder|FetchOpenOrders|WatchOrders|WatchPositions|\\.env" .claude/skills/adding-exchange-adapters
```

Expected: all references are linked from `SKILL.md`, and terminology matches current repository contracts.

- [ ] **Step 4: Cross-check the references against Backpack implementation details**

Manually verify the new references are consistent with the Backpack work that motivated them:

- `sdk-boundaries.md` reflects repository-native SDK layout choices rather than a canonical tree
- `order-semantics.md` does not invent new adapter APIs beyond current `exchange.go`
- `private-streams-and-localstate.md` matches `LocalState.Start` behavior and Backpack unsupported-stream handling
- `live-test-wiring.md` matches the resilient `.env` lookup and shared-suite pattern used by Backpack

- [ ] **Step 5: Commit the reference set**

```bash
git add .claude/skills/adding-exchange-adapters
git commit -m "docs: add adapter skill reference guides"
```

### Task 4: Pressure-Test And Finalize The Skill

**Files:**
- Read: `.claude/skills/adding-exchange-adapters/SKILL.md`
- Read: `.claude/skills/adding-exchange-adapters/references/*.md`
- Read: `docs/superpowers/specs/2026-03-22-adding-exchange-adapters-skill-design.md`

- [ ] **Step 1: Re-run the pressure scenario against the updated skill**

Run: dispatch one fresh review/pressure subagent with the updated skill and the same scenario from Task 1

Fallback if subagents are unavailable: manually answer the same four questions using only the updated skill files, then compare the answers line by line against the approved spec.

Expected: the subagent can now answer the four critical questions with repository-accurate guidance:

- when to create `sdk/`
- how `FetchOrder` differs from `FetchOpenOrders`
- what makes private stream support real
- how to wire live `testsuite` coverage

- [ ] **Step 2: Fix any remaining ambiguity**

If the pressure test still shows gaps, make the smallest documentation edits needed to close them, keeping the main skill short and pushing detail into references.

- [ ] **Step 3: Run final verification commands**

Run:

```bash
git diff --check
sed -n '1,260p' .claude/skills/adding-exchange-adapters/SKILL.md
find .claude/skills/adding-exchange-adapters/references -maxdepth 1 -type f | sort
rg -n "Before You Write Code|Architecture Decisions|Order Contract Checklist|Private API Readiness|Live Test Readiness|Do Not Ship If" .claude/skills/adding-exchange-adapters/SKILL.md
```

Expected:

- no whitespace or patch-format issues
- main skill present and readable
- all four reference files exist
- required main-skill sections are present

- [ ] **Step 4: Run a spec checklist pass**

Re-read `docs/superpowers/specs/2026-03-22-adding-exchange-adapters-skill-design.md` and verify each of these is clearly satisfied in the final docs:

- main skill complements `exchanges` instead of duplicating it
- reference docs exist for SDK boundaries, order semantics, private streams/local state, and live test wiring
- unsupported shared surfaces are routed to `exchanges.ErrNotSupported`
- Backpack-driven lessons are reflected in the new guidance
- `.env` lookup convention is resilient to worktrees

- [ ] **Step 5: Commit final refinements**

```bash
git add .claude/skills/adding-exchange-adapters
git commit -m "docs: finalize adapter skill expansion"
```
