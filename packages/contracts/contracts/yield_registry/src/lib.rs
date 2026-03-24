#![no_std]

use soroban_sdk::{contract, contractimpl, contracttype, symbol_short, Address, Env, Symbol};

use nester_access_control::{self as ac, Role};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/// Lifecycle status of a yield source.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SourceStatus {
    Active,
    Paused,
}

#[contracttype]
#[derive(Clone)]
enum DataKey {
    Source(Symbol),
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct YieldRegistryContract;

#[contractimpl]
impl YieldRegistryContract {
    /// Initialise the registry, granting `admin` the Admin role.
    pub fn initialize(env: Env, admin: Address) {
        ac::initialize(&env, &admin);
    }

    // -----------------------------------------------------------------------
    // Source management — Admin only
    // -----------------------------------------------------------------------

    /// Register or update the lifecycle status of a yield source.
    ///
    /// Requires caller to hold [`Role::Admin`].
    pub fn upsert_source(env: Env, caller: Address, source_id: Symbol, status: SourceStatus) {
        caller.require_auth();
        ac::require_role(&env, &caller, Role::Admin);

        env.storage()
            .instance()
            .set(&DataKey::Source(source_id.clone()), &status);

        env.events()
            .publish((symbol_short!("src_ups"), caller, source_id), ());
    }

    /// Return `true` if the source has ever been registered (regardless of status).
    pub fn has_source(env: Env, source_id: Symbol) -> bool {
        env.storage()
            .instance()
            .has(&DataKey::Source(source_id))
    }

    /// Return the current [`SourceStatus`] for `source_id`.
    ///
    /// Panics if the source does not exist.
    pub fn get_source_status(env: Env, source_id: Symbol) -> SourceStatus {
        env.storage()
            .instance()
            .get(&DataKey::Source(source_id))
            .unwrap_or_else(|| {
                soroban_sdk::panic_with_error!(&env, nester_common::ContractError::StrategyNotFound)
            })
    }

    // -----------------------------------------------------------------------
    // Role management — delegates to nester_access_control
    // -----------------------------------------------------------------------

    /// Grant `role` to `grantee`. Caller must be an Admin.
    pub fn grant_role(env: Env, grantor: Address, grantee: Address, role: Role) {
        ac::grant_role(&env, &grantor, &grantee, role);
    }

    /// Revoke `role` from `target`. Caller must be an Admin.
    pub fn revoke_role(env: Env, revoker: Address, target: Address, role: Role) {
        ac::revoke_role(&env, &revoker, &target, role);
    }

    /// Propose an admin transfer (step 1). Caller must be an Admin.
    pub fn transfer_admin(env: Env, current_admin: Address, new_admin: Address) {
        ac::transfer_admin(&env, &current_admin, &new_admin);
    }

    /// Accept a pending admin transfer (step 2). Caller must be the proposed new admin.
    pub fn accept_admin(env: Env, new_admin: Address) {
        ac::accept_admin(&env, &new_admin);
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod test;
