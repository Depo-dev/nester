# require_auth audit (Issue #1132)

## Finding

`allocation_strategy`, `treasury`, `yield_registry`, and `vault` each had a
`propose_upgrade` and `cancel_upgrade` entrypoint that called
`AccessControl::require_role(&env, &admin, Role::Upgrader)` **without**
first calling `admin.require_auth()`. `require_role` only checks that the
given address holds a role in storage — it does not verify the caller
actually authorized as that address. Every other admin/role-gated
entrypoint in this codebase calls `require_auth()` immediately before the
matching `require_role`/`require_role_typed` check (see e.g.
`treasury::withdraw_internal`, `vault::rebalance_internal`,
`allocation_strategy::set_weights_internal`) — these eight functions were
the exception, not an intentional design choice.

Practical impact: anyone could call `propose_upgrade`/`cancel_upgrade` on
these four contracts, passing the real Upgrader's address as the `admin`
parameter, without that address's signature — scheduling or cancelling a
contract upgrade with no authorization from the actual role holder.

`execute_upgrade` on these same contracts is unaffected — it's
intentionally permissionless after a proposal matures (documented as such
in each contract already), which is a deliberate design choice, not a gap.

## Fix

Added `admin.require_auth()` as the first line of all eight functions,
matching the pattern used everywhere else in the codebase.

## Verification method

Found via a script enumerating every `pub fn` taking a caller-shaped
`Address` parameter and checking whether `require_auth` appears in its body
(resolving one level of `Self::..._internal` delegation), then manually
verifying each candidate — most were false positives resolved by a shared
library call (`AccessControl::initialize`, `Timelock::propose`, etc.) that
already calls `require_auth` internally.

Regression coverage: extended the existing
`privileged_strategy_calls_require_signatures` test in
`allocation_strategy` (which clears all mocked auths and asserts every
privileged call fails) to also cover `propose_upgrade`/`cancel_upgrade`.

## Scope not covered by this pass

This audit found and fixed one specific, confirmed pattern (role-gated
functions missing the auth check for the role's holder) using automated
enumeration and manual verification. It is not a claim that every
entrypoint in every contract has been individually enumerated with its
intended authorization documented, nor does it add a CI check that fails
when a new entrypoint lacks a matching auth test — both are real, larger
follow-up work.
