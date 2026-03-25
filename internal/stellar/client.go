package stellar

import (
	"context"
	"fmt"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// Client wraps the Stellar SDK and provides domain-specific operations
type Client struct {
	config        Config
	horizon       *horizonclient.Client
	networkID     string
	sourceAccount *txnbuild.SimpleAccount
}

// NewClient initializes a Stellar client with the given configuration
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Network == "" {
		return nil, fmt.Errorf("network is required")
	}
	if cfg.RPCURL == "" {
		return nil, fmt.Errorf("RPC URL is required")
	}
	if cfg.SourceKey == "" {
		return nil, fmt.Errorf("source key is required")
	}

	// Set defaults
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 100
	}

	// Initialize Horizon client for queries
	horizon := &horizonclient.Client{
		HorizonURL: cfg.RPCURL,
	}

	// Validate connection
	if _, err := horizon.Root(); err != nil {
		return nil, fmt.Errorf("failed to connect to Stellar network: %w", err)
	}

	// Set network ID based on network selection
	networkID := getNetworkID(cfg.Network)
	if cfg.NetworkID != "" {
		networkID = cfg.NetworkID
	}

	client := &Client{
		config:    cfg,
		horizon:   horizon,
		networkID: networkID,
	}

	// Validate source key can be used for signing
	if err := validateSourceKey(cfg.SourceKey); err != nil {
		return nil, fmt.Errorf("invalid source key: %w", err)
	}

	return client, nil
}

// Health performs a health check on the Stellar connection
func (c *Client) Health(ctx context.Context) (*HealthCheck, error) {
	start := time.Now()

	_, err := c.horizon.Root()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &HealthCheck{
			Healthy:        false,
			NetworkLatency: latency,
			LastCheckTime:  time.Now().Unix(),
			Error:          err.Error(),
		}, nil
	}

	return &HealthCheck{
		Healthy:        true,
		NetworkLatency: latency,
		LastCheckTime:  time.Now().Unix(),
	}, nil
}

// getNetworkID returns the network passphrase for the given network
func getNetworkID(n Network) string {
	switch n {
	case Testnet:
		return network.TestNetworkPassphrase
	case Mainnet:
		return network.PublicNetworkPassphrase
	case Futurenet:
		return network.FutureNetworkPassphrase
	default:
		return network.TestNetworkPassphrase
	}
}

// validateSourceKey validates that the source key is a valid Stellar key
func validateSourceKey(key string) error {
	// Basic validation - Stellar keys are typically 56 characters starting with S
	if len(key) != 56 {
		return fmt.Errorf("invalid key length: expected 56, got %d", len(key))
	}
	if key[0] != 'S' && key[0] != 'G' {
		return fmt.Errorf("invalid key prefix: must start with S (secret) or G (public)")
	}
	return nil
}

// Close gracefully closes the client connection
func (c *Client) Close() error {
	// Horizon client doesn't require explicit cleanup, but we keep this
	// for potential future connection pooling
	return nil
}
