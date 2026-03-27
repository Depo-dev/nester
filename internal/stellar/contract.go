package stellar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// ContractInvoker handles all contract invocations and simulations
type ContractInvoker struct {
	client *Client
	logger *log.Entry
}

// NewContractInvoker creates a new contract invoker
func NewContractInvoker(client *Client) *ContractInvoker {
	return &ContractInvoker{
		client: client,
		logger: log.WithField("component", "contract_invoker"),
	}
}

// SimulateContract simulates a contract invocation without submitting it
func (ci *ContractInvoker) SimulateContract(
	ctx context.Context,
	contractID string,
	method string,
	args []interface{},
) (*SimulationResult, error) {
	// Build the transaction that would call the contract
	tx, err := ci.buildContractInvocation(ctx, contractID, method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build contract invocation: %w", err)
	}

	// Simulate the transaction
	result := &SimulationResult{
		IsSuccess: false,
	}

	// Call Soroban RPC to simulate (this is a placeholder for the actual implementation)
	// In production, this would call the actual Soroban RPC simulateTransaction endpoint
	if tx == nil {
		return nil, fmt.Errorf("failed to build transaction")
	}

	result.IsSuccess = true
	result.GasEstimate = 10000 // Placeholder - would be returned from simulation
	return result, nil
}

// InvokeContract invokes a contract method and submits the transaction
func (ci *ContractInvoker) InvokeContract(
	ctx context.Context,
	contractID string,
	method string,
	args []interface{},
) (*ContractResult, error) {
	// First simulate to catch errors early
	sim, err := ci.SimulateContract(ctx, contractID, method, args)
	if err != nil {
		return nil, fmt.Errorf("simulation failed: %w", err)
	}

	if !sim.IsSuccess {
		return nil, fmt.Errorf("simulation failed: %s", sim.Error)
	}

	// Build the transaction
	tx, err := ci.buildContractInvocation(ctx, contractID, method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build contract invocation: %w", err)
	}

	// Submit with retries
	result, err := ci.submitWithRetries(ctx, tx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// buildContractInvocation builds a Soroban contract invocation transaction
func (ci *ContractInvoker) buildContractInvocation(
	ctx context.Context,
	contractID string,
	method string,
	args []interface{},
) (*txnbuild.Transaction, error) {
	if err := validateContractID(contractID); err != nil {
		return nil, err
	}
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	if err := validateSorobanArgs(args); err != nil {
		return nil, fmt.Errorf("invalid contract arguments: %w", err)
	}

	// Note: This is a simplified implementation. In production, this would:
	// 1. Create proper XDR representations of arguments
	// 2. Build a complete InvokeHostFunction operation
	// 3. Handle authorization structures if needed

	// Placeholder implementation
	_ = ci.client.networkID

	// This would normally return a properly constructed transaction
	// For now, returning nil as we'll need the actual Soroban RPC setup
	return nil, nil
}

func validateContractID(contractID string) error {
	if contractID == "" {
		return fmt.Errorf("contract ID is required")
	}
	if len(contractID) != 56 || contractID[0] != 'C' {
		return fmt.Errorf("contract ID must be a 56-character Stellar contract address starting with C")
	}
	return nil
}

func validateSorobanArgs(args []interface{}) error {
	for index, arg := range args {
		if err := validateSorobanArg(arg); err != nil {
			return fmt.Errorf("arg %d: %w", index, err)
		}
	}
	return nil
}

func validateSorobanArg(arg interface{}) error {
	switch value := arg.(type) {
	case nil, bool, string, []byte,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return nil
	case []interface{}:
		for index, nested := range value {
			if err := validateSorobanArg(nested); err != nil {
				return fmt.Errorf("vec[%d]: %w", index, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported argument type %T", arg)
	}
}

// submitWithRetries submits a transaction with exponential backoff retry logic
func (ci *ContractInvoker) submitWithRetries(ctx context.Context, tx *txnbuild.Transaction) (*ContractResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	maxRetries := ci.client.config.MaxRetries
	backoff := time.Duration(ci.client.config.RetryBackoff) * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
			// Exponential backoff
			backoff *= 2
		}

		result, err := ci.submitTransaction(ctx, tx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if the error is retryable
		if !isRetryableError(err) {
			return &ContractResult{
				IsSuccess: false,
				Error:     err.Error(),
			}, nil
		}

		ci.logger.WithField("attempt", attempt+1).
			WithField("error", err).
			Debug("Retrying transaction submission")
	}

	return nil, fmt.Errorf("failed to submit transaction after %d retries: %w", maxRetries, lastErr)
}

// submitTransaction submits a transaction to the network
func (ci *ContractInvoker) submitTransaction(ctx context.Context, tx *txnbuild.Transaction) (*ContractResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	// This would normally:
	// 1. Sign the transaction with the source key
	// 2. Submit to Soroban RPC or Horizon
	// 3. Poll for confirmation

	// Placeholder result
	result := &ContractResult{
		IsSuccess:       true,
		TransactionHash: "0x" + "00", // Placeholder
		BlockNumber:     0,
	}

	return result, nil
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network-related errors that should be retried
	retryableErrors := []string{
		"timeout",
		"temporary failure",
		"rate limited",
		"503",
		"502",
		"connection refused",
		"i/o timeout",
	}

	for _, retryable := range retryableErrors {
		if len(errStr) > 0 && contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}
