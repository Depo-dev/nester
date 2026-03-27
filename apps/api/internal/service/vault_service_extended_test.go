package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/suncrestlabs/nester/apps/api/internal/domain/vault"
)

func TestVaultServiceCreateVaultStoresAndReturnsCorrectly(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	input := CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-TEST-001",
		Currency:        "USDC",
	}

	created, err := service.CreateVault(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	if created.ID == uuid.Nil {
		t.Fatal("created vault should have non-nil ID")
	}
	if created.UserID != userID {
		t.Fatalf("created vault UserID = %v, want %v", created.UserID, userID)
	}
	if created.ContractAddress != "CA-TEST-001" {
		t.Fatalf("created vault ContractAddress = %q, want %q", created.ContractAddress, "CA-TEST-001")
	}
	if created.Currency != "USDC" {
		t.Fatalf("created vault Currency = %q, want %q", created.Currency, "USDC")
	}
	if created.Status != vault.StatusActive {
		t.Fatalf("created vault Status = %q, want %q", created.Status, vault.StatusActive)
	}
	if !created.TotalDeposited.Equal(decimal.Zero) {
		t.Fatalf("created vault TotalDeposited = %s, want 0", created.TotalDeposited)
	}
	if !created.CurrentBalance.Equal(decimal.Zero) {
		t.Fatalf("created vault CurrentBalance = %s, want 0", created.CurrentBalance)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("created vault should have non-zero CreatedAt")
	}
	if created.UpdatedAt.IsZero() {
		t.Fatal("created vault should have non-zero UpdatedAt")
	}

	// Verify it's stored in repository
	fetched, err := repository.GetVault(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetVault() error = %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("fetched vault ID = %v, want %v", fetched.ID, created.ID)
	}
}

func TestVaultServiceCreateVaultWithCustomStatus(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	input := CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-TEST-002",
		Currency:        "USDC",
		Status:          "paused",
	}

	created, err := service.CreateVault(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	if created.Status != vault.StatusPaused {
		t.Fatalf("created vault Status = %q, want %q", created.Status, vault.StatusPaused)
	}
}

func TestVaultServiceCreateVaultInvalidInputs(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	tests := []struct {
		name  string
		input CreateVaultInput
		err   error
	}{
		{
			name: "empty user ID",
			input: CreateVaultInput{
				UserID:          uuid.Nil,
				ContractAddress: "CA-001",
				Currency:        "USDC",
			},
			err: vault.ErrInvalidVault,
		},
		{
			name: "empty contract address",
			input: CreateVaultInput{
				UserID:          userID,
				ContractAddress: "",
				Currency:        "USDC",
			},
			err: vault.ErrInvalidVault,
		},
		{
			name: "whitespace contract address",
			input: CreateVaultInput{
				UserID:          userID,
				ContractAddress: "   ",
				Currency:        "USDC",
			},
			err: vault.ErrInvalidVault,
		},
		{
			name: "empty currency",
			input: CreateVaultInput{
				UserID:          userID,
				ContractAddress: "CA-001",
				Currency:        "",
			},
			err: vault.ErrInvalidVault,
		},
		{
			name: "invalid status",
			input: CreateVaultInput{
				UserID:          userID,
				ContractAddress: "CA-001",
				Currency:        "USDC",
				Status:          "invalid",
			},
			err: vault.ErrInvalidVault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateVault(context.Background(), tt.input)
			if err != tt.err {
				t.Fatalf("CreateVault() error = %v, want %v", err, tt.err)
			}
		})
	}
}

func TestVaultServiceGetVaultReturnsVaultOrNotFound(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-GET-001",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test successful get
	fetched, err := service.GetVault(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetVault() error = %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("GetVault() ID = %v, want %v", fetched.ID, created.ID)
	}

	// Test not found
	_, err = service.GetVault(context.Background(), uuid.New())
	if err != vault.ErrVaultNotFound {
		t.Fatalf("GetVault() error = %v, want %v", err, vault.ErrVaultNotFound)
	}

	// Test invalid ID
	_, err = service.GetVault(context.Background(), uuid.Nil)
	if err != vault.ErrInvalidVault {
		t.Fatalf("GetVault() error = %v, want %v", err, vault.ErrInvalidVault)
	}
}

func TestVaultServiceGetUserVaultsReturnsAllActiveVaults(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()
	repository := newMemoryVaultRepository(userID, otherUserID)
	service := NewVaultService(repository)

	// Create vaults for user
	vault1, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-USER-001",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	vault2, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-USER-002",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Create vault for other user
	_, err = service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          otherUserID,
		ContractAddress: "CA-OTHER-001",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Get user vaults
	vaults, err := service.GetUserVaults(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserVaults() error = %v", err)
	}

	if len(vaults) != 2 {
		t.Fatalf("GetUserVaults() returned %d vaults, want 2", len(vaults))
	}

	// Verify both vaults are present
	vaultIDs := make(map[uuid.UUID]bool)
	for _, v := range vaults {
		vaultIDs[v.ID] = true
	}
	if !vaultIDs[vault1.ID] {
		t.Fatal("GetUserVaults() missing vault1")
	}
	if !vaultIDs[vault2.ID] {
		t.Fatal("GetUserVaults() missing vault2")
	}
}

func TestVaultServiceGetUserVaultsInvalidInput(t *testing.T) {
	repository := newMemoryVaultRepository()
	service := NewVaultService(repository)

	_, err := service.GetUserVaults(context.Background(), uuid.Nil)
	if err != vault.ErrInvalidVault {
		t.Fatalf("GetUserVaults() error = %v, want %v", err, vault.ErrInvalidVault)
	}
}

func TestVaultServiceUpdateAllocationsValidatesWeightSum(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-ALLOC-001",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test allocations that sum to exactly 100%
	validAllocations := []vault.Allocation{
		{Protocol: "aave", Amount: decimal.RequireFromString("40"), APY: decimal.RequireFromString("4.5")},
		{Protocol: "blend", Amount: decimal.RequireFromString("60"), APY: decimal.RequireFromString("5.2")},
	}

	updated, err := service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID:     created.ID,
		Allocations: validAllocations,
	})
	if err != nil {
		t.Fatalf("UpdateAllocations() error = %v", err)
	}

	if len(updated.Allocations) != 2 {
		t.Fatalf("UpdateAllocations() returned %d allocations, want 2", len(updated.Allocations))
	}

	// Verify total amount sums to 100
	totalAmount := decimal.Zero
	for _, alloc := range updated.Allocations {
		totalAmount = totalAmount.Add(alloc.Amount)
	}
	if !totalAmount.Equal(decimal.RequireFromString("100")) {
		t.Fatalf("allocation amounts sum to %s, want 100", totalAmount)
	}
}

func TestVaultServiceUpdateAllocationsRejectsInvalidWeightSum(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-ALLOC-002",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test allocations that don't sum to 100%
	invalidAllocations := []vault.Allocation{
		{Protocol: "aave", Amount: decimal.RequireFromString("40"), APY: decimal.RequireFromString("4.5")},
		{Protocol: "blend", Amount: decimal.RequireFromString("50"), APY: decimal.RequireFromString("5.2")},
	}

	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID:     created.ID,
		Allocations: invalidAllocations,
	})
	if err != vault.ErrInvalidAllocation {
		t.Fatalf("UpdateAllocations() error = %v, want %v", err, vault.ErrInvalidAllocation)
	}
}

func TestVaultServiceUpdateAllocationsValidatesProtocol(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-ALLOC-003",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test empty protocol
	invalidAllocations := []vault.Allocation{
		{Protocol: "", Amount: decimal.RequireFromString("100"), APY: decimal.RequireFromString("4.5")},
	}

	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID:     created.ID,
		Allocations: invalidAllocations,
	})
	if err != vault.ErrInvalidAllocation {
		t.Fatalf("UpdateAllocations() error = %v, want %v", err, vault.ErrInvalidAllocation)
	}
}

func TestVaultServiceUpdateAllocationsValidatesAmount(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-ALLOC-004",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test negative amount
	invalidAllocations := []vault.Allocation{
		{Protocol: "aave", Amount: decimal.RequireFromString("-10"), APY: decimal.RequireFromString("4.5")},
	}

	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID:     created.ID,
		Allocations: invalidAllocations,
	})
	if err != vault.ErrInvalidAllocation {
		t.Fatalf("UpdateAllocations() error = %v, want %v", err, vault.ErrInvalidAllocation)
	}
}

func TestVaultServiceUpdateAllocationsValidatesAPY(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-ALLOC-005",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test negative APY
	invalidAllocations := []vault.Allocation{
		{Protocol: "aave", Amount: decimal.RequireFromString("100"), APY: decimal.RequireFromString("-1")},
	}

	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID:     created.ID,
		Allocations: invalidAllocations,
	})
	if err != vault.ErrInvalidAllocation {
		t.Fatalf("UpdateAllocations() error = %v, want %v", err, vault.ErrInvalidAllocation)
	}
}

func TestVaultServiceUpdateAllocationsInvalidVaultID(t *testing.T) {
	repository := newMemoryVaultRepository()
	service := NewVaultService(repository)

	_, err := service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID: uuid.Nil,
		Allocations: []vault.Allocation{
			{Protocol: "aave", Amount: decimal.RequireFromString("100"), APY: decimal.RequireFromString("4.5")},
		},
	})
	if err != vault.ErrInvalidVault {
		t.Fatalf("UpdateAllocations() error = %v, want %v", err, vault.ErrInvalidVault)
	}
}

func TestVaultServiceUpdateAllocationsNormalizesProtocol(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-ALLOC-006",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test protocol normalization (uppercase to lowercase)
	allocations := []vault.Allocation{
		{Protocol: "AAVE", Amount: decimal.RequireFromString("50"), APY: decimal.RequireFromString("4.5")},
		{Protocol: "BLEND", Amount: decimal.RequireFromString("50"), APY: decimal.RequireFromString("5.2")},
	}

	updated, err := service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID:     created.ID,
		Allocations: allocations,
	})
	if err != nil {
		t.Fatalf("UpdateAllocations() error = %v", err)
	}

	// Verify protocols are normalized to lowercase
	for _, alloc := range updated.Allocations {
		if alloc.Protocol != "aave" && alloc.Protocol != "blend" {
			t.Fatalf("protocol %q not normalized to lowercase", alloc.Protocol)
		}
	}
}

func TestVaultServiceRecordDepositValidatesAmount(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-DEPOSIT-001",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Test zero amount
	_, err = service.RecordDeposit(context.Background(), RecordDepositInput{
		VaultID: created.ID,
		Amount:  decimal.Zero,
	})
	if err != vault.ErrInvalidAmount {
		t.Fatalf("RecordDeposit() error = %v, want %v", err, vault.ErrInvalidAmount)
	}

	// Test negative amount
	_, err = service.RecordDeposit(context.Background(), RecordDepositInput{
		VaultID: created.ID,
		Amount:  decimal.RequireFromString("-10"),
	})
	if err != vault.ErrInvalidAmount {
		t.Fatalf("RecordDeposit() error = %v, want %v", err, vault.ErrInvalidAmount)
	}

	// Test invalid vault ID
	_, err = service.RecordDeposit(context.Background(), RecordDepositInput{
		VaultID: uuid.Nil,
		Amount:  decimal.RequireFromString("100"),
	})
	if err != vault.ErrInvalidVault {
		t.Fatalf("RecordDeposit() error = %v, want %v", err, vault.ErrInvalidVault)
	}
}

func TestVaultServiceRecordDepositUpdatesBalances(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	created, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-DEPOSIT-002",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// First deposit
	updated, err := service.RecordDeposit(context.Background(), RecordDepositInput{
		VaultID: created.ID,
		Amount:  decimal.RequireFromString("100.50"),
	})
	if err != nil {
		t.Fatalf("RecordDeposit() error = %v", err)
	}

	if !updated.TotalDeposited.Equal(decimal.RequireFromString("100.50")) {
		t.Fatalf("TotalDeposited = %s, want 100.50", updated.TotalDeposited)
	}
	if !updated.CurrentBalance.Equal(decimal.RequireFromString("100.50")) {
		t.Fatalf("CurrentBalance = %s, want 100.50", updated.CurrentBalance)
	}

	// Second deposit
	updated, err = service.RecordDeposit(context.Background(), RecordDepositInput{
		VaultID: created.ID,
		Amount:  decimal.RequireFromString("50.25"),
	})
	if err != nil {
		t.Fatalf("RecordDeposit() error = %v", err)
	}

	if !updated.TotalDeposited.Equal(decimal.RequireFromString("150.75")) {
		t.Fatalf("TotalDeposited = %s, want 150.75", updated.TotalDeposited)
	}
	if !updated.CurrentBalance.Equal(decimal.RequireFromString("150.75")) {
		t.Fatalf("CurrentBalance = %s, want 150.75", updated.CurrentBalance)
	}
}

func TestListVaults_SortedByAPYDescending(t *testing.T) {
	userID := uuid.New()
	repository := newMemoryVaultRepository(userID)
	service := NewVaultService(repository)

	// Create vaults with different APYs
	vault1, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-APY-001",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	vault2, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-APY-002",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	vault3, err := service.CreateVault(context.Background(), CreateVaultInput{
		UserID:          userID,
		ContractAddress: "CA-APY-003",
		Currency:        "USDC",
	})
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Add allocations with different APYs
	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID: vault1.ID,
		Allocations: []vault.Allocation{
			{Protocol: "aave", Amount: decimal.RequireFromString("100"), APY: decimal.RequireFromString("3.5")},
		},
	})
	if err != nil {
		t.Fatalf("UpdateAllocations() error = %v", err)
	}

	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID: vault2.ID,
		Allocations: []vault.Allocation{
			{Protocol: "blend", Amount: decimal.RequireFromString("100"), APY: decimal.RequireFromString("5.2")},
		},
	})
	if err != nil {
		t.Fatalf("UpdateAllocations() error = %v", err)
	}

	_, err = service.UpdateAllocations(context.Background(), UpdateAllocationsInput{
		VaultID: vault3.ID,
		Allocations: []vault.Allocation{
			{Protocol: "compound", Amount: decimal.RequireFromString("100"), APY: decimal.RequireFromString("4.1")},
		},
	})
	if err != nil {
		t.Fatalf("UpdateAllocations() error = %v", err)
	}

	// Get user vaults
	vaults, err := service.GetUserVaults(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserVaults() error = %v", err)
	}

	if len(vaults) != 3 {
		t.Fatalf("GetUserVaults() returned %d vaults, want 3", len(vaults))
	}

	// Verify vaults are sorted by APY descending
	// vault2 has APY 5.2, vault3 has APY 4.1, vault1 has APY 3.5
	if vaults[0].ID != vault2.ID {
		t.Fatalf("first vault should be vault2 (APY 5.2), got %v", vaults[0].ID)
	}
	if vaults[1].ID != vault3.ID {
		t.Fatalf("second vault should be vault3 (APY 4.1), got %v", vaults[1].ID)
	}
	if vaults[2].ID != vault1.ID {
		t.Fatalf("third vault should be vault1 (APY 3.5), got %v", vaults[2].ID)
	}

	// Verify APY values are correct
	if len(vaults[0].Allocations) == 0 || !vaults[0].Allocations[0].APY.Equal(decimal.RequireFromString("5.2")) {
		t.Fatalf("first vault APY = %v, want 5.2", vaults[0].Allocations[0].APY)
	}
	if len(vaults[1].Allocations) == 0 || !vaults[1].Allocations[0].APY.Equal(decimal.RequireFromString("4.1")) {
		t.Fatalf("second vault APY = %v, want 4.1", vaults[1].Allocations[0].APY)
	}
	if len(vaults[2].Allocations) == 0 || !vaults[2].Allocations[0].APY.Equal(decimal.RequireFromString("3.5")) {
		t.Fatalf("third vault APY = %v, want 3.5", vaults[2].Allocations[0].APY)
	}
}
