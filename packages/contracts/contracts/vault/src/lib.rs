#![no_std]

use soroban_sdk::{
    contract, contractimpl, contracttype, panic_with_error, symbol_short, token, Address, Env,
};

use nester_access_control::{AccessControl, Role};
use nester_common::ContractError;

// ---------------------------------------------------------------------------
// Storage
// ---------------------------------------------------------------------------

#[contracttype]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum VaultStatus {
    Active,
    Paused,
}

#[contracttype]
#[derive(Clone)]
enum DataKey {
    Token,
    Status,
    Balance(Address),
    TotalDeposits,
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn require_initialized(env: &Env) {
    if !env.storage().instance().has(&DataKey::Token) {
        panic_with_error!(env, ContractError::NotInitialized);
    }
}

fn require_active(env: &Env) {
    let status: VaultStatus = env
        .storage()
        .instance()
        .get(&DataKey::Status)
        .unwrap_or(VaultStatus::Paused);
    if status != VaultStatus::Active {
        panic_with_error!(env, ContractError::InvalidOperation);
    }
}

fn get_balance(env: &Env, user: &Address) -> i128 {
    env.storage()
        .persistent()
        .get(&DataKey::Balance(user.clone()))
        .unwrap_or(0)
}

fn set_balance(env: &Env, user: &Address, amount: i128) {
    env.storage()
        .persistent()
        .set(&DataKey::Balance(user.clone()), &amount);
}

fn get_total(env: &Env) -> i128 {
    env.storage()
        .instance()
        .get(&DataKey::TotalDeposits)
        .unwrap_or(0)
}

fn set_total(env: &Env, amount: i128) {
    env.storage()
        .instance()
        .set(&DataKey::TotalDeposits, &amount);
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct VaultContract;

#[contractimpl]
impl VaultContract {
    /// Initialise the vault, setting `admin` as the sole Admin.
    pub fn initialize(env: Env, admin: Address, token_address: Address) {
        // AccessControl::initialize handles AlreadyInitialized guard and require_auth
        AccessControl::initialize(&env, &admin);
        env.storage()
            .instance()
            .set(&DataKey::Token, &token_address);
        env.storage()
            .instance()
            .set(&DataKey::Status, &VaultStatus::Active);
        env.storage()
            .instance()
            .set(&DataKey::TotalDeposits, &0_i128);
    }

    // -----------------------------------------------------------------------
    // Admin operations
    // -----------------------------------------------------------------------

    /// Pause all vault operations. Requires [`Role::Admin`].
    pub fn pause(env: Env, caller: Address) {
        require_initialized(&env);
        caller.require_auth();
        AccessControl::require_role(&env, &caller, Role::Admin);
        env.storage()
            .instance()
            .set(&DataKey::Status, &VaultStatus::Paused);
        env.events()
            .publish((symbol_short!("paused"), caller), ());
    }

    /// Resume vault operations. Requires [`Role::Admin`].
    pub fn unpause(env: Env, caller: Address) {
        require_initialized(&env);
        caller.require_auth();
        AccessControl::require_role(&env, &caller, Role::Admin);
        env.storage()
            .instance()
            .set(&DataKey::Status, &VaultStatus::Active);
        env.events()
            .publish((symbol_short!("unpaused"), caller), ());
    }

    /// Grant `role` to `grantee`. Requires caller to be an Admin.
    pub fn grant_role(env: Env, grantor: Address, grantee: Address, role: Role) {
        AccessControl::grant_role(&env, &grantor, &grantee, role);
    }

    /// Revoke `role` from `target`. Requires caller to be an Admin.
    pub fn revoke_role(env: Env, revoker: Address, target: Address, role: Role) {
        AccessControl::revoke_role(&env, &revoker, &target, role);
    }

    /// Propose an admin transfer (step 1). Requires caller to be an Admin.
    pub fn transfer_admin(env: Env, current_admin: Address, new_admin: Address) {
        AccessControl::transfer_admin(&env, &current_admin, &new_admin);
    }

    /// Accept a proposed admin transfer (step 2). Caller must be the pending new admin.
    pub fn accept_admin(env: Env, new_admin: Address) {
        AccessControl::accept_admin(&env, &new_admin);
    }

    // -----------------------------------------------------------------------
    // Core vault operations
    // -----------------------------------------------------------------------

    /// Deposit funds into the vault.
    pub fn deposit(env: Env, user: Address, amount: i128) -> i128 {
        require_initialized(&env);
        require_active(&env);

        if amount <= 0 {
            panic_with_error!(&env, ContractError::InvalidAmount);
        }

        user.require_auth();

        let token_address: Address = env.storage().instance().get(&DataKey::Token).unwrap();
        let contract_address = env.current_contract_address();

        token::Client::new(&env, &token_address).transfer(&user, &contract_address, &amount);

        let new_balance = get_balance(&env, &user) + amount;
        set_balance(&env, &user, new_balance);

        let new_total = get_total(&env) + amount;
        set_total(&env, new_total);

        env.events().publish(
            (symbol_short!("DEPOSIT"), user.clone()),
            (amount, new_balance),
        );

        new_balance
    }

    /// Withdraw funds from the vault.
    pub fn withdraw(env: Env, user: Address, amount: i128) -> i128 {
        require_initialized(&env);

        if amount <= 0 {
            panic_with_error!(&env, ContractError::InvalidAmount);
        }

        user.require_auth();

        let current_balance = get_balance(&env, &user);
        if amount > current_balance {
            panic_with_error!(&env, ContractError::InsufficientBalance);
        }

        let token_address: Address = env.storage().instance().get(&DataKey::Token).unwrap();
        let contract_address = env.current_contract_address();

        token::Client::new(&env, &token_address).transfer(&contract_address, &user, &amount);

        let new_balance = current_balance - amount;
        set_balance(&env, &user, new_balance);

        let new_total = get_total(&env) - amount;
        set_total(&env, new_total);

        env.events().publish(
            (symbol_short!("WITHDRAW"), user.clone()),
            (amount, new_balance),
        );

        new_balance
    }

    // -----------------------------------------------------------------------
    // View functions
    // -----------------------------------------------------------------------

    pub fn get_balance(env: Env, user: Address) -> i128 {
        require_initialized(&env);
        get_balance(&env, &user)
    }

    pub fn get_total_deposits(env: Env) -> i128 {
        require_initialized(&env);
        get_total(&env)
    }

    pub fn get_status(env: Env) -> VaultStatus {
        require_initialized(&env);
        env.storage()
            .instance()
            .get(&DataKey::Status)
            .unwrap_or(VaultStatus::Paused)
    }

    pub fn get_token(env: Env) -> Address {
        require_initialized(&env);
        env.storage().instance().get(&DataKey::Token).unwrap()
    }

    pub fn is_paused(env: Env) -> bool {
        env.storage()
            .instance()
            .get::<_, VaultStatus>(&DataKey::Status)
            .map(|s| s == VaultStatus::Paused)
            .unwrap_or(true)
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod test;
