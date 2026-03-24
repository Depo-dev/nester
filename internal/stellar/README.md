# Stellar/Soroban Integration Layer

Production-ready Go integration layer for Stellar and Soroban smart contracts. This package abstracts the complexity of the Stellar Go SDK behind clean, domain-oriented interfaces.

## Features

- **Stellar Client**: Network initialization, health checks, connection management
- **Contract Invocations**: Simulate and submit Soroban contract methods with proper error handling
- **Vault Reader**: Type-safe access to vault contract data (balances, allocations)
- **Event Polling**: Subscribe to and stream Soroban contract events
- **Retry Logic**: Exponential backoff with configurable max retries for transient failures
- **Type Safety**: All SDK/XDR types are converted to domain types; no leakage into business logic

## Architecture

```
internal/stellar/
├── types.go           # Domain types (VaultBalance, Allocation, Event, etc.)
├── client.go          # Stellar client initialization & health checks
├── contract.go        # Contract invocation & simulation with retry logic
├── vault_reader.go    # Type-safe vault query methods
├── events.go          # Event polling and streaming
└── *_test.go         # Comprehensive test coverage
```

## Usage

### Initialize Client

```go
package main

import (
	"context"
	"github.com/Damola09/nester/internal/stellar"
)

func main() {
	cfg := stellar.Config{
		Network:      stellar.Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	client, err := stellar.NewClient(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// Client is ready to use
}
```

### Query Vault Balance

```go
reader := stellar.NewVaultReader(invoker)

balance, err := reader.GetVaultBalance(ctx, contractID)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Total: %s, Available: %s, Locked: %s\n",
	balance.Total, balance.Available, balance.Locked)
```

### Invoke Contract Method

```go
invoker := stellar.NewContractInvoker(client)

result, err := invoker.InvokeContract(
	ctx,
	contractID,
	"deposit",
	[]interface{}{"USDC", amount},
)

if err != nil {
	log.Fatal(err)
}

fmt.Printf("Transaction Hash: %s\n", result.TransactionHash)
```

### Simulate Contract Call

```go
simResult, err := invoker.SimulateContract(
	ctx,
	contractID,
	"get_balance",
	[]interface{}{},
)

if err != nil {
	log.Fatal(err)
}

fmt.Printf("Gas Estimate: %d\n", simResult.GasEstimate)
```

### Subscribe to Events

```go
poller := stellar.NewEventPoller(client)

err := poller.Subscribe(contractID, func(event *stellar.Event) {
	fmt.Printf("Event: %s on contract %s\n", event.EventType, event.ContractID)
})

if err != nil {
	log.Fatal(err)
}

// Start polling in background
go poller.WatchEvents(ctx, contractID, 5*time.Second)

// Later, stop polling
poller.Stop()
```

### Stream Events

```go
stream := poller.NewEventStream(ctx, contractID, 5*time.Second)

for {
	select {
	case event := <-stream.Events:
		fmt.Printf("Got event: %s\n", event.EventType)
	case err := <-stream.Errors:
		log.Fatal(err)
	case <-ctx.Done():
		return
	}
}
```

## Error Handling

All operations return errors for:
- **Network Errors**: Automatically retried with exponential backoff (up to max retries)
- **Validation Errors**: Empty contract IDs, nil listeners, invalid key formats (returned immediately)
- **Contract Errors**: Simulation/invocation failures (returned immediately with error message)

Retryable errors include:
- Connection timeouts
- Rate limiting (429, 503, 502)
- Temporary network failures
- I/O timeouts

## Configuration

```go
type Config struct {
	Network      Network // Testnet, Mainnet, or Futurenet
	RPCURL       string  // RPC endpoint URL
	SourceKey    string  // Private key for signing (56 chars, starts with S)
	NetworkID    string  // Optional: Network passphrase (auto-set from Network)
	ContractID   string  // Optional: Default contract ID
	MaxRetries   int     // Max retries for transient failures (default: 3)
	RetryBackoff int     // Initial backoff in milliseconds (default: 100)
}
```

### Environment Configuration

```go
// Load from environment
cfg := stellar.Config{
	Network:      stellar.Network(os.Getenv("STELLAR_NETWORK")),
	RPCURL:       os.Getenv("STELLAR_RPC_URL"),
	SourceKey:    os.Getenv("STELLAR_SOURCE_KEY"),
	MaxRetries:   3,
	RetryBackoff: 100,
}
```

## Type Safety

All domain types are defined in `types.go`:
- `VaultBalance`: Vault state with Total, Available, Locked amounts
- `Allocation`: Yield source allocation with amount and APY
- `Event`: Contract event with typed data
- `HealthCheck`: Network connection status

These types have no dependency on Stellar SDK/XDR types, ensuring clean separation of concerns.

## Testing

All components are fully tested with comprehensive unit tests:

```bash
go test ./internal/stellar/... -v
```

### Integration Tests

For live testnet testing (skipped by default in CI):

```bash
go test -run TestIntegration ./internal/stellar/... -v
```

## Production Checklist

- ✅ Stellar client connects and validates network
- ✅ Contract simulation works end-to-end
- ✅ RPC failures retry with exponential backoff (max 3 attempts)
- ✅ All Stellar types converted to domain types
- ✅ Network config is environment-driven
- ✅ Health checks available for readiness probes
- ✅ Event polling foundation for future indexer
- ✅ Comprehensive error handling
- ✅ Full test coverage with mocks

## Future Enhancements

- [ ] Custom RPC client for direct Soroban RPC communication
- [ ] Event indexing and caching
- [ ] Transaction status polling and receipt management
- [ ] Batch contract invocations
- [ ] Multi-sig transaction support
- [ ] Gas estimation and fee calculation
