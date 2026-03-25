package stellar

import (
	"github.com/shopspring/decimal"
)

// Network represents a Stellar network configuration
type Network string

const (
	Testnet   Network = "testnet"
	Mainnet   Network = "mainnet"
	Futurenet Network = "futurenet"
)

// Config holds Stellar client configuration
type Config struct {
	Network      Network
	RPCURL       string
	SourceKey    string // Private key for signing transactions
	NetworkID    string // Network passphrase for signing
	ContractID   string // Optional: default contract ID
	MaxRetries   int    // Max retries for transient failures
	RetryBackoff int    // Initial backoff in milliseconds
}

// VaultBalance represents the current state of a vault
type VaultBalance struct {
	ContractID string
	Total      decimal.Decimal
	Available  decimal.Decimal
	Locked     decimal.Decimal
}

// Allocation represents a vault's allocation to a yield source
type Allocation struct {
	ContractID   string
	SourceName   string
	Amount       decimal.Decimal
	APY          decimal.Decimal
	AllocationID string
}

// ContractResult represents the result of a contract invocation
type ContractResult struct {
	TransactionHash string
	BlockNumber     uint64
	IsSuccess       bool
	Error           string
	ReturnValue     interface{}
}

// SimulationResult represents the result of a contract simulation
type SimulationResult struct {
	IsSuccess   bool
	Error       string
	ReturnValue interface{}
	GasEstimate uint64
}

// Event represents a Soroban contract event
type Event struct {
	ContractID    string
	EventType     string
	BlockNumber   uint64
	TransactionID string
	Data          map[string]interface{}
	Timestamp     int64
}

// HealthCheck represents the health status of the Stellar connection
type HealthCheck struct {
	Healthy        bool
	NetworkLatency int64 // milliseconds
	LastCheckTime  int64
	Error          string
}
