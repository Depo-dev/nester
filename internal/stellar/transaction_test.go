package stellar

import (
	"context"
	"errors"
	"testing"

	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Transaction Submission Tests (Unit & Integration)
// ============================================================================

func TestSubmitTransaction_ReturnsPlaceholderSuccess(t *testing.T) {
	invoker := NewContractInvoker(&Client{config: Config{MaxRetries: 3, RetryBackoff: 100}, networkID: getNetworkID(Testnet)})

	result, err := invoker.submitTransaction(context.Background(), &txnbuild.Transaction{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsSuccess)
	assert.Equal(t, "0x00", result.TransactionHash)
}

func TestSubmitWithRetries_ReturnsSuccessOnFirstAttempt(t *testing.T) {
	invoker := NewContractInvoker(&Client{config: Config{MaxRetries: 3, RetryBackoff: 100}, networkID: getNetworkID(Testnet)})

	result, err := invoker.submitWithRetries(context.Background(), &txnbuild.Transaction{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsSuccess)
}

func TestSubmitTransaction_ReturnsErrorForNilTransaction(t *testing.T) {
	invoker := NewContractInvoker(&Client{config: Config{MaxRetries: 3, RetryBackoff: 100}, networkID: getNetworkID(Testnet)})

	result, err := invoker.submitTransaction(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "transaction is nil")
}

func TestTransactionSubmission_RetryableErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{name: "timeout", err: errors.New("i/o timeout"), retryable: true},
		{name: "uppercase connection refused", err: errors.New("CONNECTION REFUSED"), retryable: true},
		{name: "temporary failure", err: errors.New("temporary failure"), retryable: true},
		{name: "permanent validation error", err: errors.New("invalid contract ID"), retryable: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, isRetryableError(tt.err))
		})
	}
}

func TestTransactionSubmission_NilTransactionReturnsError(t *testing.T) {
	invoker := NewContractInvoker(&Client{config: Config{MaxRetries: 3, RetryBackoff: 100}, networkID: getNetworkID(Testnet)})

	_, err := invoker.submitWithRetries(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction is nil")
}
