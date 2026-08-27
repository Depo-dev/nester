package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/suncrestlabs/nester/apps/api/internal/auth"
)

func TestWalletBindingCheck_MatchingWallet(t *testing.T) {
	token := makeToken(t, auth.Claims{
		Subject:       "user-1",
		WalletAddress: walletA,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
	})

	handler := Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(
		WalletBindingCheck(ok200))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/wallet/"+walletA, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("matching wallet: got %d, want 200", rec.Code)
	}
}

func TestWalletBindingCheck_CrossWalletRejection(t *testing.T) {
	token := makeToken(t, auth.Claims{
		Subject:       "user-1",
		WalletAddress: walletA,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
	})

	handler := Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(
		WalletBindingCheck(ok200))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/wallet/"+walletB, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-wallet: got %d, want 403", rec.Code)
	}
}

func TestWalletBindingCheck_NoWalletInPath(t *testing.T) {
	token := makeToken(t, auth.Claims{
		Subject:       "user-1",
		WalletAddress: walletA,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
	})

	handler := Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(
		WalletBindingCheck(ok200))

	// Routes without a wallet address in the path should pass through.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaults", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("no wallet in path: got %d, want 200", rec.Code)
	}
}

func TestWalletBindingCheck_CaseInsensitive(t *testing.T) {
	upper := "GCEZWKCA5VLDNRLN3RPRJMRZOX3Z6G5CHCGZP1WKU56V25HXQOPJFHM"
	lower := "gcezwkca5vldnrln3rprjmrzox3z6g5chcgzp1wku56v25hxqopjfhm"

	token := makeToken(t, auth.Claims{
		Subject:       "user-1",
		WalletAddress: upper,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
	})

	handler := Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(
		WalletBindingCheck(ok200))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/wallet/"+lower, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("case-insensitive: got %d, want 200", rec.Code)
	}
}

func TestWalletBindingCheck_PublicRouteSkipped(t *testing.T) {
	// Public routes don't carry a user context, so wallet binding is skipped.
	handler := Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(
		WalletBindingCheck(ok200))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("public route: got %d, want 200", rec.Code)
	}
}

func TestWalletBindingCheck_EmptyWalletAddress(t *testing.T) {
	// A token with no wallet address should pass through (no binding to check).
	token := makeToken(t, auth.Claims{
		Subject:   "user-1",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})

	handler := Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(
		WalletBindingCheck(ok200))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/wallet/"+walletA, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("empty wallet in token: got %d, want 200", rec.Code)
	}
}
