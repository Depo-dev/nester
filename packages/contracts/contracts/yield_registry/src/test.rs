#![cfg(test)]

extern crate std;

use super::*;
use soroban_sdk::{
    symbol_short,
    testutils::{Address as _, Events},
    Address, Env,
};

#[test]
fn upsert_and_read_source_status() {
    let env = Env::default();
    env.mock_all_auths();

    let admin = Address::generate(&env);
    let contract_id = env.register_contract(None, YieldRegistryContract);
    let client = YieldRegistryContractClient::new(&env, &contract_id);

    client.initialize(&admin);
    client.upsert_source(&admin, &symbol_short!("aave"), &SourceStatus::Active);

    assert!(client.has_source(&symbol_short!("aave")));
    assert_eq!(
        client.get_source_status(&symbol_short!("aave")),
        SourceStatus::Active
    );
    assert!(!env.events().all().is_empty());
}

#[test]
fn only_admin_can_upsert_source() {
    let env = Env::default();
    env.mock_all_auths();

    let admin = Address::generate(&env);
    let outsider = Address::generate(&env);
    let contract_id = env.register_contract(None, YieldRegistryContract);
    let client = YieldRegistryContractClient::new(&env, &contract_id);

    client.initialize(&admin);

    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        client.upsert_source(&outsider, &symbol_short!("aave"), &SourceStatus::Active);
    }));

    assert!(result.is_err());
}

#[test]
fn upsert_source_overwrites_status() {
    let env = Env::default();
    env.mock_all_auths();

    let admin = Address::generate(&env);
    let contract_id = env.register_contract(None, YieldRegistryContract);
    let client = YieldRegistryContractClient::new(&env, &contract_id);

    client.initialize(&admin);
    client.upsert_source(&admin, &symbol_short!("aave"), &SourceStatus::Active);
    assert_eq!(client.get_source_status(&symbol_short!("aave")), SourceStatus::Active);

    client.upsert_source(&admin, &symbol_short!("aave"), &SourceStatus::Paused);
    assert_eq!(client.get_source_status(&symbol_short!("aave")), SourceStatus::Paused);
}

#[test]
fn has_source_returns_false_for_unregistered() {
    let env = Env::default();
    env.mock_all_auths();

    let admin = Address::generate(&env);
    let contract_id = env.register_contract(None, YieldRegistryContract);
    let client = YieldRegistryContractClient::new(&env, &contract_id);

    client.initialize(&admin);
    assert!(!client.has_source(&symbol_short!("ghost")));
}

#[test]
fn admin_transfer_works_in_registry() {
    let env = Env::default();
    env.mock_all_auths();

    let admin = Address::generate(&env);
    let new_admin = Address::generate(&env);
    let contract_id = env.register_contract(None, YieldRegistryContract);
    let client = YieldRegistryContractClient::new(&env, &contract_id);

    client.initialize(&admin);
    client.transfer_admin(&admin, &new_admin);
    client.accept_admin(&new_admin);

    // New admin can upsert; old cannot.
    client.upsert_source(&new_admin, &symbol_short!("aave"), &SourceStatus::Active);
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        client.upsert_source(&admin, &symbol_short!("aave"), &SourceStatus::Paused);
    }));
    assert!(result.is_err());
}
