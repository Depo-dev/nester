#![no_std]

extern crate alloc;

use alloc::vec;
use alloc::vec::Vec as RustVec;

use soroban_sdk::{
    contract, contractimpl, contracttype, panic_with_error, symbol_short, Address, Env, Symbol, Vec,
};

use nester_access_control::{AccessControl, Role};
use nester_common::{ContractError, BASIS_POINT_SCALE};
use yield_registry::{SourceStatus, YieldRegistryContractClient};

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AllocationWeight {
    pub source_id: Symbol,
    pub weight_bps: u32,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SourceApy {
    pub source_id: Symbol,
    pub apy_bps: i128,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum VaultType {
    Conservative,
    Balanced,
    Growth,
    DeFi500,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StrategyParams {
    pub rebalance_threshold_bps: u32,
    pub max_weight_bps: u32,
}

#[contracttype]
#[derive(Clone)]
enum DataKey {
    RegistryId,
    VaultType,
    Weights,
    Allocation(Symbol),
    RebalanceThresholdBps,
    MaxWeightBps,
}

#[contract]
pub struct AllocationStrategyContract;

#[contractimpl]
impl AllocationStrategyContract {
    pub fn initialize(env: Env, admin: Address, registry_id: Address) {
        Self::initialize_with_vault_type(env, admin, registry_id, VaultType::Balanced);
    }

    pub fn initialize_with_vault_type(
        env: Env,
        admin: Address,
        registry_id: Address,
        vault_type: VaultType,
    ) {
        AccessControl::initialize(&env, &admin);
        env.storage().instance().set(&DataKey::RegistryId, &registry_id);
        env.storage().instance().set(&DataKey::VaultType, &vault_type);

        let params = default_strategy_params(&vault_type);
        env.storage()
            .instance()
            .set(&DataKey::RebalanceThresholdBps, &params.rebalance_threshold_bps);
        env.storage()
            .instance()
            .set(&DataKey::MaxWeightBps, &params.max_weight_bps);

        let default_weights = build_default_weights(&env, &registry_id, &vault_type);
        env.storage().instance().set(&DataKey::Weights, &default_weights);
    }

    pub fn get_vault_type(env: Env) -> VaultType {
        env.storage().instance().get(&DataKey::VaultType).unwrap()
    }

    pub fn get_strategy_params(env: Env) -> StrategyParams {
        StrategyParams {
            rebalance_threshold_bps: env.storage().instance().get(&DataKey::RebalanceThresholdBps).unwrap(),
            max_weight_bps: env.storage().instance().get(&DataKey::MaxWeightBps).unwrap(),
        }
    }

    pub fn update_strategy_params(
        env: Env,
        caller: Address,
        rebalance_threshold_bps: u32,
        max_weight_bps: u32,
    ) {
        caller.require_auth();
        AccessControl::require_role(&env, &caller, Role::Admin);

        if rebalance_threshold_bps > BASIS_POINT_SCALE
            || max_weight_bps == 0
            || max_weight_bps > BASIS_POINT_SCALE
        {
            panic_with_error!(&env, ContractError::InvalidOperation);
        }

        env.storage()
            .instance()
            .set(&DataKey::RebalanceThresholdBps, &rebalance_threshold_bps);
        env.storage().instance().set(&DataKey::MaxWeightBps, &max_weight_bps);
    }

    pub fn set_weights(env: Env, caller: Address, weights: Vec<AllocationWeight>) {
        caller.require_auth();
        require_admin_or_operator(&env, &caller);

        validate_weight_sum(&env, &weights);

        let registry_id: Address = env.storage().instance().get(&DataKey::RegistryId).unwrap();
        let registry = YieldRegistryContractClient::new(&env, &registry_id);

        for weight in weights.iter() {
            if !registry.has_source(&weight.source_id) {
                panic_with_error!(&env, ContractError::StrategyNotFound);
            }
            if registry.get_source_status(&weight.source_id) != SourceStatus::Active {
                panic_with_error!(&env, ContractError::InvalidOperation);
            }
        }

        env.storage().instance().set(&DataKey::Weights, &weights);
        env.events().publish((symbol_short!("wts_set"), caller), ());
    }

    pub fn get_weights(env: Env) -> Vec<AllocationWeight> {
        env.storage()
            .instance()
            .get(&DataKey::Weights)
            .unwrap_or_else(|| Vec::new(&env))
    }

    pub fn compute_allocation(
        env: Env,
        total_amount: i128,
        apys: Vec<SourceApy>,
    ) -> Vec<AllocationWeight> {
        let registry_id: Address = env.storage().instance().get(&DataKey::RegistryId).unwrap();
        let registry = YieldRegistryContractClient::new(&env, &registry_id);
        let vault_type = Self::get_vault_type(env.clone());
        let params = Self::get_strategy_params(env.clone());

        let mut results = rust_weights_from_entries(&apys);
        let mut eligible_indices = RustVec::new();
        let mut scores = RustVec::new();

        for (index, entry) in apys.iter().enumerate() {
            let is_registered = registry.has_source(&entry.source_id);
            let is_active = is_registered
                && registry.get_source_status(&entry.source_id) == SourceStatus::Active;

            if is_active && entry.apy_bps > 0 {
                eligible_indices.push(index as usize);
                let score = match vault_type {
                    VaultType::DeFi500 => 1_i128,
                    _ => entry.apy_bps,
                };
                scores.push(score);
            }
        }

        if !eligible_indices.is_empty() {
            let computed = match vault_type {
                VaultType::DeFi500 => even_distribution(eligible_indices.len()),
                _ => proportional_with_cap(&env, &scores, params.max_weight_bps),
            };

            for (slot, index) in eligible_indices.iter().enumerate() {
                results[*index].weight_bps = computed[slot];
            }
        }

        let weights = soroban_weights_from_rust(&env, &results);
        env.storage().instance().set(&DataKey::Weights, &weights);
        persist_allocations(&env, total_amount, &weights);
        weights
    }

    pub fn calculate_allocation(env: Env, total: i128) -> Vec<(Symbol, i128)> {
        let weights = Self::get_weights(env.clone());
        let allocations = allocation_amounts(&weights, total);

        for (symbol, amount) in allocations.iter() {
            env.storage()
                .instance()
                .set(&DataKey::Allocation(symbol.clone()), amount);
        }

        let mut out = Vec::new(&env);
        for (symbol, amount) in allocations {
            out.push_back((symbol, amount));
        }
        out
    }

    pub fn needs_rebalance(
        env: Env,
        current_weights: Vec<AllocationWeight>,
        target_weights: Vec<AllocationWeight>,
    ) -> bool {
        let threshold = Self::get_strategy_params(env).rebalance_threshold_bps;
        let mut seen = RustVec::new();

        for weight in current_weights.iter() {
            if !contains_symbol(&seen, &weight.source_id) {
                seen.push(weight.source_id.clone());
            }
        }
        for weight in target_weights.iter() {
            if !contains_symbol(&seen, &weight.source_id) {
                seen.push(weight.source_id.clone());
            }
        }

        for symbol in seen {
            let current = lookup_weight(&current_weights, &symbol);
            let target = lookup_weight(&target_weights, &symbol);
            if current.abs_diff(target) > threshold {
                return true;
            }
        }

        false
    }

    pub fn get_source_allocation(env: Env, source_id: Symbol) -> i128 {
        env.storage()
            .instance()
            .get(&DataKey::Allocation(source_id))
            .unwrap_or(0_i128)
    }

    pub fn grant_role(env: Env, grantor: Address, grantee: Address, role: Role) {
        AccessControl::grant_role(&env, &grantor, &grantee, role);
    }

    pub fn revoke_role(env: Env, revoker: Address, target: Address, role: Role) {
        AccessControl::revoke_role(&env, &revoker, &target, role);
    }

    pub fn transfer_admin(env: Env, current_admin: Address, new_admin: Address) {
        AccessControl::transfer_admin(&env, &current_admin, &new_admin);
    }

    pub fn accept_admin(env: Env, new_admin: Address) {
        AccessControl::accept_admin(&env, &new_admin);
    }
}

fn require_admin_or_operator(env: &Env, account: &Address) {
    if !AccessControl::has_role(env, account, Role::Admin)
        && !AccessControl::has_role(env, account, Role::Operator)
    {
        panic_with_error!(env, ContractError::Unauthorized);
    }
}

fn default_strategy_params(vault_type: &VaultType) -> StrategyParams {
    match vault_type {
        VaultType::Conservative => StrategyParams {
            rebalance_threshold_bps: 250,
            max_weight_bps: 5_000,
        },
        VaultType::Balanced => StrategyParams {
            rebalance_threshold_bps: 500,
            max_weight_bps: 6_500,
        },
        VaultType::Growth => StrategyParams {
            rebalance_threshold_bps: 750,
            max_weight_bps: 8_500,
        },
        VaultType::DeFi500 => StrategyParams {
            rebalance_threshold_bps: 100,
            max_weight_bps: 10_000,
        },
    }
}

fn build_default_weights(env: &Env, registry_id: &Address, vault_type: &VaultType) -> Vec<AllocationWeight> {
    let registry = YieldRegistryContractClient::new(env, registry_id);
    let active_sources = registry.get_active_sources();
    let mut source_ids = RustVec::new();

    for source in active_sources.iter() {
        source_ids.push(source.id);
    }

    let distribution = match vault_type {
        VaultType::Conservative => template_distribution(source_ids.len(), &[5_000, 3_000, 2_000]),
        VaultType::Balanced => template_distribution(source_ids.len(), &[4_000, 3_500, 2_500]),
        VaultType::Growth => template_distribution(source_ids.len(), &[2_000, 3_000, 5_000]),
        VaultType::DeFi500 => even_distribution(source_ids.len()),
    };

    let mut out = Vec::new(env);
    for (index, source_id) in source_ids.into_iter().enumerate() {
        out.push_back(AllocationWeight {
            source_id,
            weight_bps: distribution[index],
        });
    }

    out
}

fn template_distribution(count: usize, template: &[u32]) -> RustVec<u32> {
    if count == 0 {
        return RustVec::new();
    }
    if count == 1 {
        return vec![BASIS_POINT_SCALE];
    }
    if count == 2 {
        return vec![template[0] + (template[1] / 2), template[2] + (template[1] / 2)];
    }

    let mut out = vec![0_u32; count];
    out[0] = template[0];
    out[1] = template[1];
    out[2] = template[2];
    out
}

fn even_distribution(count: usize) -> RustVec<u32> {
    if count == 0 {
        return RustVec::new();
    }

    let base = BASIS_POINT_SCALE / count as u32;
    let remainder = BASIS_POINT_SCALE % count as u32;
    let mut out = vec![base; count];

    for weight in out.iter_mut().take(remainder as usize) {
        *weight += 1;
    }

    out
}

fn proportional_with_cap(env: &Env, scores: &[i128], max_weight_bps: u32) -> RustVec<u32> {
    if scores.is_empty() {
        return RustVec::new();
    }

    if max_weight_bps as usize * scores.len() < BASIS_POINT_SCALE as usize {
        panic_with_error!(env, ContractError::AllocationError);
    }

    let len = scores.len();
    let mut assigned = vec![0_u32; len];
    let mut active = vec![true; len];
    let mut remaining_total = BASIS_POINT_SCALE;

    while remaining_total > 0 {
        let mut total_score = 0_i128;
        for index in 0..len {
            if active[index] {
                total_score += scores[index];
            }
        }

        if total_score == 0 {
            break;
        }

        let snapshot_remaining = remaining_total;

        let mut floors = vec![0_u32; len];
        let mut remainders = vec![0_i128; len];
        let mut capped_any = false;

        for index in 0..len {
            if !active[index] {
                continue;
            }

            let capacity = max_weight_bps - assigned[index];
            let numerator = scores[index] * snapshot_remaining as i128;
            let floor = (numerator / total_score) as u32;
            let ceil = ((numerator + total_score - 1) / total_score) as u32;
            floors[index] = floor;
            remainders[index] = numerator % total_score;

            if ceil >= capacity {
                assigned[index] += capacity;
                remaining_total -= capacity;
                active[index] = false;
                capped_any = true;
            }
        }

        if capped_any {
            continue;
        }

        let mut distributed = 0_u32;
        for index in 0..len {
            if !active[index] {
                continue;
            }
            assigned[index] += floors[index];
            distributed += floors[index];
        }

        remaining_total -= distributed;

        while remaining_total > 0 {
            let mut best_index = None;
            let mut best_remainder = -1_i128;

            for index in 0..len {
                if !active[index] || assigned[index] >= max_weight_bps {
                    continue;
                }
                if remainders[index] > best_remainder {
                    best_remainder = remainders[index];
                    best_index = Some(index);
                }
            }

            match best_index {
                Some(index) => {
                    assigned[index] += 1;
                    remaining_total -= 1;
                }
                None => break,
            }
        }

        break;
    }

    assigned
}

fn rust_weights_from_entries(apys: &Vec<SourceApy>) -> RustVec<AllocationWeight> {
    let mut out = RustVec::new();
    for entry in apys.iter() {
        out.push(AllocationWeight {
            source_id: entry.source_id,
            weight_bps: 0,
        });
    }
    out
}

fn soroban_weights_from_rust(env: &Env, weights: &[AllocationWeight]) -> Vec<AllocationWeight> {
    let mut out = Vec::new(env);
    for weight in weights {
        out.push_back(weight.clone());
    }
    out
}

fn validate_weight_sum(env: &Env, weights: &Vec<AllocationWeight>) {
    let mut sum = 0_u32;
    for weight in weights.iter() {
        sum += weight.weight_bps;
    }
    if sum != BASIS_POINT_SCALE {
        panic_with_error!(env, ContractError::AllocationError);
    }
}

fn persist_allocations(env: &Env, total_amount: i128, weights: &Vec<AllocationWeight>) {
    for (symbol, amount) in allocation_amounts(weights, total_amount) {
        env.storage().instance().set(&DataKey::Allocation(symbol), &amount);
    }
}

fn allocation_amounts(weights: &Vec<AllocationWeight>, total_amount: i128) -> RustVec<(Symbol, i128)> {
    let scale = BASIS_POINT_SCALE as i128;
    let mut out = RustVec::new();
    let mut total_allocated = 0_i128;
    let mut max_index = None;
    let mut max_weight = 0_u32;

    for (index, weight) in weights.iter().enumerate() {
        let amount = (total_amount * weight.weight_bps as i128) / scale;
        total_allocated += amount;
        if weight.weight_bps > max_weight {
            max_weight = weight.weight_bps;
            max_index = Some(index as usize);
        }
        out.push((weight.source_id, amount));
    }

    if let Some(index) = max_index {
        let remainder = total_amount - total_allocated;
        if remainder > 0 {
            out[index].1 += remainder;
        }
    }

    out
}

fn contains_symbol(symbols: &[Symbol], target: &Symbol) -> bool {
    for symbol in symbols {
        if symbol == target {
            return true;
        }
    }
    false
}

fn lookup_weight(weights: &Vec<AllocationWeight>, target: &Symbol) -> u32 {
    for weight in weights.iter() {
        if weight.source_id == *target {
            return weight.weight_bps;
        }
    }
    0
}

#[cfg(test)]
mod test;
