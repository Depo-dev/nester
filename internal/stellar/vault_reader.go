package stellar

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// VaultReader provides type-safe access to vault contract data
type VaultReader struct {
	invoker *ContractInvoker
}

// NewVaultReader creates a new vault reader
func NewVaultReader(invoker *ContractInvoker) *VaultReader {
	return &VaultReader{
		invoker: invoker,
	}
}

// GetVaultBalance queries the total balance of a vault
func (vr *VaultReader) GetVaultBalance(ctx context.Context, contractID string) (*VaultBalance, error) {
	if contractID == "" {
		return nil, fmt.Errorf("contract ID is required")
	}

	// Simulate a call to the vault's balance_of or get_balance method
	result, err := vr.invoker.SimulateContract(ctx, contractID, "get_balance", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to query vault balance: %w", err)
	}

	if !result.IsSuccess {
		return nil, fmt.Errorf("balance query failed: %s", result.Error)
	}

	// Convert the result to a domain type
	balance := &VaultBalance{
		ContractID: contractID,
		Total:      decimal.Zero,
		Available:  decimal.Zero,
		Locked:     decimal.Zero,
	}

	// In production, we would parse the XDR result and convert to decimal
	// For now, this is a placeholder that shows the pattern
	if result.ReturnValue != nil {
		// Parse result.ReturnValue and populate balance fields
		// This depends on the actual contract return format
	}

	return balance, nil
}

// GetVaultAllocations queries the allocations of a vault across yield sources
func (vr *VaultReader) GetVaultAllocations(ctx context.Context, contractID string) ([]Allocation, error) {
	if contractID == "" {
		return nil, fmt.Errorf("contract ID is required")
	}

	// Simulate a call to the vault's get_allocations method
	result, err := vr.invoker.SimulateContract(ctx, contractID, "get_allocations", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to query allocations: %w", err)
	}

	if !result.IsSuccess {
		return nil, fmt.Errorf("allocations query failed: %s", result.Error)
	}

	var allocations []Allocation

	// In production, we would:
	// 1. Parse the XDR result
	// 2. Extract allocation data
	// 3. Convert to domain types
	// 4. Return as a slice

	// Parse result.ReturnValue and populate allocations
	if result.ReturnValue != nil {
		// This would iterate through the returned allocation list
		// and convert each one to our domain Allocation type
	}

	return allocations, nil
}

// GetAllocationDetails queries specific allocation details
func (vr *VaultReader) GetAllocationDetails(
	ctx context.Context,
	contractID string,
	allocationID string,
) (*Allocation, error) {
	if contractID == "" {
		return nil, fmt.Errorf("contract ID is required")
	}
	if allocationID == "" {
		return nil, fmt.Errorf("allocation ID is required")
	}

	// Call get_allocation_details with the allocation ID
	result, err := vr.invoker.SimulateContract(
		ctx,
		contractID,
		"get_allocation_details",
		[]interface{}{allocationID},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query allocation details: %w", err)
	}

	if !result.IsSuccess {
		return nil, fmt.Errorf("allocation query failed: %s", result.Error)
	}

	allocation := &Allocation{
		ContractID:   contractID,
		AllocationID: allocationID,
	}

	// Parse result.ReturnValue and populate allocation fields
	if result.ReturnValue != nil {
		// Extract source name, amount, and APY from the result
	}

	return allocation, nil
}

// VerifyVaultIntegrity performs sanity checks on vault state
func (vr *VaultReader) VerifyVaultIntegrity(ctx context.Context, contractID string) (bool, error) {
	if contractID == "" {
		return false, fmt.Errorf("contract ID is required")
	}

	// Query balance
	balance, err := vr.GetVaultBalance(ctx, contractID)
	if err != nil {
		return false, fmt.Errorf("failed to verify vault balance: %w", err)
	}

	// Basic sanity checks
	if balance == nil {
		return false, fmt.Errorf("balance is nil")
	}

	// Total should be >= available and >= locked
	if balance.Total.LessThan(balance.Available) || balance.Total.LessThan(balance.Locked) {
		return false, fmt.Errorf("invalid balance state: total < available or total < locked")
	}

	// Available + locked should equal or be less than total (accounting for pending transfers)
	sum := balance.Available.Add(balance.Locked)
	if sum.GreaterThan(balance.Total) {
		return false, fmt.Errorf("invalid balance state: available + locked > total")
	}

	return true, nil
}
