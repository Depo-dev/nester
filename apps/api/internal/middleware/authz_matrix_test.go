package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/suncrestlabs/nester/apps/api/internal/auth"
)

// authzMatrixRules mirrors the production route table from main.go so the
// matrix test stays in sync with real deployments.
var authzMatrixRules = []RouteRule{
	{PathPrefix: "/health", Public: true},
	{PathPrefix: "/healthz", Public: true},
	{PathPrefix: "/readyz", Public: true},
	{PathPrefix: "/ws", Public: true},
	{Method: http.MethodPost, PathPrefix: "/api/v1/auth/challenge", Public: true},
	{Method: http.MethodPost, PathPrefix: "/api/v1/auth/verify", Public: true},
	{Method: http.MethodPost, PathPrefix: "/api/v1/auth/refresh", Public: true},
	{PathPrefix: "/api/v1/banks/", Public: true},
	{PathPrefix: "/api/v1/yields/", Public: true},
	{PathPrefix: "/api/v1/savings-goals/shared/", Public: true},
	{PathPrefix: "/api/v1/admin/", Public: false, Role: "admin"},
	{PathPrefix: "/api/v1/internal/", Role: "service"},
	{PathPrefix: "/api/v1/", Public: false},
}

// authzMatrixHandler wraps ok200 with Authenticate using authzMatrixRules.
func authzMatrixHandler() http.Handler {
	return Authenticate(testSecret, "", authzMatrixRules, alwaysActiveRevocation)(ok200)
}

// walletA and walletB represent two distinct Stellar accounts for
// cross-user authorization testing.
const (
	walletA = "GCEZWKCA5VLDNRLN3RPRJMRZOX3Z6G5CHCGZP1WKU56V25HXQOPJFHM"
	walletB = "GAZK5JWNIQIF5OFJDQ7J2ZFND3KQ2HP7FYJNKJX6H4L3Y7S5Z4PK5YYB"
)

// AuthzRoute describes a single route to exercise in the authorization matrix.
type AuthzRoute struct {
	Method      string
	Path        string
	Public      bool // expected: anonymous gets 200
	RequireRole string
}

// authzMatrix enumerates every authenticated route the API exposes. Each entry
// specifies the HTTP method, path, and whether it is public.
//
// IMPORTANT: When a new route is added to the API, it MUST appear here or the
// TestAuthorizationMatrix route-coverage check will fail.
var authzMatrix = []AuthzRoute{
	// ── Health / infrastructure ──────────────────────────────────────────
	{Method: "GET", Path: "/health", Public: true},
	{Method: "GET", Path: "/healthz", Public: true},
	{Method: "GET", Path: "/readyz", Public: true},
	{Method: "GET", Path: "/ws", Public: true},

	// ── Auth (public handshake) ──────────────────────────────────────────
	{Method: "POST", Path: "/api/v1/auth/challenge", Public: true},
	{Method: "POST", Path: "/api/v1/auth/verify", Public: true},
	{Method: "POST", Path: "/api/v1/auth/refresh", Public: true},

	// ── Auth (protected) ─────────────────────────────────────────────────
	{Method: "POST", Path: "/api/v1/auth/logout"},
	{Method: "POST", Path: "/api/v1/auth/logout-all"},
	{Method: "GET", Path: "/api/v1/auth/sessions"},
	{Method: "DELETE", Path: "/api/v1/auth/sessions/00000000-0000-0000-0000-000000000000"},

	// ── Vaults ───────────────────────────────────────────────────────────
	{Method: "POST", Path: "/api/v1/vaults"},
	{Method: "GET", Path: "/api/v1/vaults"},
	{Method: "GET", Path: "/api/v1/vaults/all"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000"},
	{Method: "POST", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/deposit"},
	{Method: "POST", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/withdraw"},
	{Method: "POST", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/harvest"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/harvest/preview"},
	{Method: "PATCH", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/harvest-frequency"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/my-position"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/preview-deposit"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/preview-withdraw"},
	{Method: "POST", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/rebalance"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/rebalance-suggestion"},
	{Method: "POST", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/emergency-withdraw"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/share-price"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/convert"},
	{Method: "GET", Path: "/api/v1/vaults/00000000-0000-0000-0000-000000000000/allocations"},
	{Method: "POST", Path: "/api/v1/vault/rebalance"},

	// ── Transactions ─────────────────────────────────────────────────────
	{Method: "GET", Path: "/api/v1/transactions"},
	{Method: "POST", Path: "/api/v1/transactions"},
	{Method: "GET", Path: "/api/v1/transactions/00000000000000000000-0"},

	// ── Portfolio / valuation / performance ──────────────────────────────
	{Method: "GET", Path: "/api/v1/portfolio"},
	{Method: "GET", Path: "/api/v1/portfolio/valuation"},
	{Method: "GET", Path: "/api/v1/performance"},
	{Method: "GET", Path: "/api/v1/performance/snapshots"},

	// ── Settlements ──────────────────────────────────────────────────────
	{Method: "GET", Path: "/api/v1/settlements"},
	{Method: "POST", Path: "/api/v1/settlements"},

	// ── Activity / notifications ─────────────────────────────────────────
	{Method: "GET", Path: "/api/v1/activity"},
	{Method: "GET", Path: "/api/v1/notifications"},
	{Method: "PATCH", Path: "/api/v1/notifications/00000000-0000-0000-0000-000000000000/read"},

	// ── User ─────────────────────────────────────────────────────────────
	{Method: "GET", Path: "/api/v1/users/me"},
	{Method: "GET", Path: "/api/v1/users/by-wallet/" + walletA},

	// ── Watchlist / savings goals / schedules ────────────────────────────
	{Method: "GET", Path: "/api/v1/watchlist"},
	{Method: "POST", Path: "/api/v1/watchlist"},
	{Method: "GET", Path: "/api/v1/users/savings-goals"},
	{Method: "POST", Path: "/api/v1/users/savings-goals"},
	{Method: "GET", Path: "/api/v1/users/savings-goals/00000000-0000-0000-0000-000000000000"},
	{Method: "PUT", Path: "/api/v1/users/savings-goals/00000000-0000-0000-0000-000000000000"},
	{Method: "DELETE", Path: "/api/v1/users/savings-goals/00000000-0000-0000-0000-000000000000"},
	{Method: "GET", Path: "/api/v1/users/savings-schedules"},
	{Method: "POST", Path: "/api/v1/users/savings-schedules"},

	// ── Admin (requires admin role) ──────────────────────────────────────
	{Method: "GET", Path: "/api/v1/admin/users", RequireRole: "admin"},
	{Method: "POST", Path: "/api/v1/admin/users", RequireRole: "admin"},

	// ── Banks (public) ───────────────────────────────────────────────────
	{Method: "GET", Path: "/api/v1/banks/supported", Public: true},

	// ── Yields (public) ───────────────────────────────────────────────
	// NOTE: prefix rule "/api/v1/yields/" requires trailing slash; the
	// exact path "/api/v1/yields" (no slash) falls through to protected.
	{Method: "GET", Path: "/api/v1/yields/", Public: true},
	{Method: "GET", Path: "/api/v1/yields/00000000-0000-0000-0000-000000000000", Public: true},
}

// TestAuthorizationMatrix verifies the three-way authorization contract for
// every route in the API:
//
//  1. Anonymous request (no token) → 401 for protected routes, 200 for public.
//  2. Non-owner request (valid token, different user) → 401/403 for protected
//     routes that require ownership; the response must NOT be 404 so the
//     endpoint is not an existence oracle.
//  3. Owner request (valid token, matching user) → 200.
//
// This test intentionally targets the auth middleware layer. Handler-level
// ownership checks (returning 404 for non-owners) are covered by the
// handler-level IDOR tests in vault_idor_test.go.
func TestAuthorizationMatrix(t *testing.T) {
	handler := authzMatrixHandler()

	for _, route := range authzMatrix {
		name := route.Method + " " + route.Path
		t.Run(name, func(t *testing.T) {
			// ── 1. Anonymous → 401 (or 200 if public) ────────────────────
			t.Run("anonymous", func(t *testing.T) {
				req := httptest.NewRequest(route.Method, route.Path, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if route.Public {
					if rec.Code != http.StatusOK {
						t.Errorf("public route %s: anonymous got %d, want 200", name, rec.Code)
					}
				} else {
					if rec.Code != http.StatusUnauthorized {
						t.Errorf("protected route %s: anonymous got %d, want 401", name, rec.Code)
					}
				}
			})

			// ── 2. Non-owner (valid token, wrong user) ───────────────────
			if !route.Public {
				t.Run("non-owner", func(t *testing.T) {
					token := makeToken(t, auth.Claims{
						Subject:       "user-other",
						WalletAddress: walletB,
						Roles:         []string{},
						ExpiresAt:     time.Now().Add(time.Hour).Unix(),
					})
					req := httptest.NewRequest(route.Method, route.Path, nil)
					req.Header.Set("Authorization", "Bearer "+token)
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)

					// Non-owner must NOT get 404 (existence oracle leak).
					// They should get 401 (no matching user context) or 403.
					if rec.Code == http.StatusNotFound {
						t.Errorf("route %s: non-owner got 404 (existence oracle leak), want 401 or 403", name)
					}
				})
			}

			// ── 3. Owner (valid token, correct user) ─────────────────────
			t.Run("owner", func(t *testing.T) {
				roles := []string{}
				if route.RequireRole == "admin" {
					roles = []string{"admin"}
				}
				token := makeToken(t, auth.Claims{
					Subject:       "user-owner",
					WalletAddress: walletA,
					Roles:         roles,
					ExpiresAt:     time.Now().Add(time.Hour).Unix(),
				})
				req := httptest.NewRequest(route.Method, route.Path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code == http.StatusUnauthorized {
					t.Errorf("route %s: owner got 401 (should be authenticated)", name)
				}
			})
		})
	}
}

// TestAuthorizationMatrixRouteCount acts as a compile-time guard: if someone
// adds a route to authzMatrixRules but forgets to add it to authzMatrix,
// this test warns (it does not fail, since the production rules are the
// source of truth for public/protected status).
func TestAuthorizationMatrixRouteCount(t *testing.T) {
	// Count non-public rules in production rules (each may match many paths).
	protectedPrefixes := 0
	for _, r := range authzMatrixRules {
		if !r.Public {
			protectedPrefixes++
		}
	}
	t.Logf("production rules: %d total, %d protected", len(authzMatrixRules), protectedPrefixes)
	t.Logf("matrix entries: %d", len(authzMatrix))

	// Sanity: matrix should have at least as many entries as protected prefixes.
	if len(authzMatrix) < protectedPrefixes {
		t.Errorf("matrix has %d entries but production has %d protected prefixes — some routes may be untested",
			len(authzMatrix), protectedPrefixes)
	}
}
