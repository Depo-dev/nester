package stellar

import (
	"context"
	"fmt"
	"time"
)

// ExampleNewClient demonstrates client initialization
func ExampleNewClient() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("Client initialized successfully")
}

// ExampleClient_Health demonstrates health checking
func ExampleClient_Health() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	health, err := client.Health(ctx)
	if err != nil {
		fmt.Printf("Health check error: %v\n", err)
		return
	}

	if health.Healthy {
		fmt.Printf("Network is healthy (latency: %dms)\n", health.NetworkLatency)
	} else {
		fmt.Printf("Network is unhealthy: %s\n", health.Error)
	}
}

// ExampleVaultReader_GetVaultBalance demonstrates querying vault balance
func ExampleVaultReader_GetVaultBalance() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	invoker := NewContractInvoker(client)
	reader := NewVaultReader(invoker)

	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"

	balance, err := reader.GetVaultBalance(ctx, contractID)
	if err != nil {
		fmt.Printf("Failed to get vault balance: %v\n", err)
		return
	}

	fmt.Printf("Vault Balance:\n")
	fmt.Printf("  Total: %s\n", balance.Total)
	fmt.Printf("  Available: %s\n", balance.Available)
	fmt.Printf("  Locked: %s\n", balance.Locked)
}

// ExampleVaultReader_GetVaultAllocations demonstrates querying allocations
func ExampleVaultReader_GetVaultAllocations() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	invoker := NewContractInvoker(client)
	reader := NewVaultReader(invoker)

	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"

	allocations, err := reader.GetVaultAllocations(ctx, contractID)
	if err != nil {
		fmt.Printf("Failed to get allocations: %v\n", err)
		return
	}

	fmt.Printf("Vault Allocations:\n")
	for _, alloc := range allocations {
		fmt.Printf("  %s: %s (APY: %s%%)\n", alloc.SourceName, alloc.Amount, alloc.APY)
	}
}

// ExampleEventPoller_Subscribe demonstrates event subscription
func ExampleEventPoller_Subscribe() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	poller := NewEventPoller(client)
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"

	// Subscribe to events
	err = poller.Subscribe(contractID, func(event *Event) {
		fmt.Printf("Event: %s at block %d\n", event.EventType, event.BlockNumber)
	})

	if err != nil {
		fmt.Printf("Failed to subscribe: %v\n", err)
		return
	}

	// Start watching events
	go func() {
		_ = poller.WatchEvents(ctx, contractID, 5*time.Second)
	}()

	// Wait a bit for events
	time.Sleep(2 * time.Second)
	poller.Stop()

	fmt.Println("Event watching stopped")
}

// ExampleEventPoller_NewEventStream demonstrates event streaming
func ExampleEventPoller_NewEventStream() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	poller := NewEventPoller(client)
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"

	stream := poller.NewEventStream(ctx, contractID, 5*time.Second)

	// Read from stream (with timeout)
	select {
	case event := <-stream.Events:
		fmt.Printf("Received event: %s\n", event.EventType)
	case err := <-stream.Errors:
		fmt.Printf("Stream error: %v\n", err)
	case <-time.After(2 * time.Second):
		fmt.Println("No events within timeout")
	}

	stream.Close()
}

// ExampleContractInvoker_SimulateContract demonstrates contract simulation
func ExampleContractInvoker_SimulateContract() {
	cfg := Config{
		Network:      Testnet,
		RPCURL:       "https://soroban-testnet.stellar.org",
		SourceKey:    "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGG",
		MaxRetries:   3,
		RetryBackoff: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	invoker := NewContractInvoker(client)
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"

	result, err := invoker.SimulateContract(ctx, contractID, "get_balance", []interface{}{})
	if err != nil {
		fmt.Printf("Simulation error: %v\n", err)
		return
	}

	if result.IsSuccess {
		fmt.Printf("Simulation succeeded (gas estimate: %d)\n", result.GasEstimate)
	} else {
		fmt.Printf("Simulation failed: %s\n", result.Error)
	}
}

// ExampleFilterEvents demonstrates event filtering
func ExampleFilterEvents() {
	events := []Event{
		{ContractID: "CONTRACT1", EventType: "Deposit", BlockNumber: 100},
		{ContractID: "CONTRACT1", EventType: "Withdrawal", BlockNumber: 101},
		{ContractID: "CONTRACT2", EventType: "Deposit", BlockNumber: 102},
	}

	deposits := FilterEvents(events, "Deposit")
	fmt.Printf("Found %d deposit events\n", len(deposits))

	contract1Events := FilterEventsByContract(events, "CONTRACT1")
	fmt.Printf("Found %d events for CONTRACT1\n", len(contract1Events))
}
