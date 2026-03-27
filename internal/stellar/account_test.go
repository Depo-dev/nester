package stellar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockHorizonRootServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"horizon_version":"2.0.0",
			"core_version":"stellar-core 20.0.0",
			"history_latest_ledger":1,
			"history_elder_ledger":1,
			"core_latest_ledger":1,
			"current_protocol_version":20,
			"network_passphrase":"Test SDF Network ; September 2015"
		}`))
		require.NoError(t, err)
	}))
}

func TestNewClient_UsesDefaultsAgainstMockHorizon(t *testing.T) {
	server := mockHorizonRootServer(t)
	defer server.Close()

	client, err := NewClient(context.Background(), Config{
		Network:   Testnet,
		RPCURL:    server.URL,
		SourceKey: "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGGX",
	})

	require.NoError(t, err)
	assert.Equal(t, 3, client.config.MaxRetries)
	assert.Equal(t, 100, client.config.RetryBackoff)
	assert.Equal(t, getNetworkID(Testnet), client.networkID)
}

func TestNewClient_UsesCustomNetworkIDAgainstMockHorizon(t *testing.T) {
	server := mockHorizonRootServer(t)
	defer server.Close()

	client, err := NewClient(context.Background(), Config{
		Network:   Testnet,
		NetworkID: "Custom Network Passphrase",
		RPCURL:    server.URL,
		SourceKey: "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGGX",
	})

	require.NoError(t, err)
	assert.Equal(t, "Custom Network Passphrase", client.networkID)
}

func TestHealth_ReturnsHealthyWithReachableHorizon(t *testing.T) {
	server := mockHorizonRootServer(t)
	defer server.Close()

	client, err := NewClient(context.Background(), Config{
		Network:   Testnet,
		RPCURL:    server.URL,
		SourceKey: "SBVH6U5PEFXPXPJ4GPXVYACRF4NZQA5QBCZLLPQGHXWWK6NXPV6IYGGX",
	})
	require.NoError(t, err)

	health, err := client.Health(context.Background())
	require.NoError(t, err)
	assert.True(t, health.Healthy)
	assert.Empty(t, health.Error)
}

func TestVaultReader_InputValidation(t *testing.T) {
	invoker := NewContractInvoker(&Client{config: Config{MaxRetries: 3, RetryBackoff: 100}, networkID: getNetworkID(Testnet)})
	reader := NewVaultReader(invoker)

	_, err := reader.GetVaultBalance(context.Background(), "")
	assert.EqualError(t, err, "contract ID is required")

	_, err = reader.GetVaultAllocations(context.Background(), "")
	assert.EqualError(t, err, "contract ID is required")

	_, err = reader.GetAllocationDetails(context.Background(), "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4", "")
	assert.EqualError(t, err, "allocation ID is required")

	ok, err := reader.VerifyVaultIntegrity(context.Background(), "")
	assert.False(t, ok)
	assert.EqualError(t, err, "contract ID is required")
}

func TestVaultReader_WrapsSimulationErrors(t *testing.T) {
	invoker := NewContractInvoker(&Client{config: Config{MaxRetries: 3, RetryBackoff: 100}, networkID: getNetworkID(Testnet)})
	reader := NewVaultReader(invoker)

	_, err := reader.GetVaultBalance(context.Background(), "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query vault balance")

	_, err = reader.GetVaultAllocations(context.Background(), "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query allocations")
}
