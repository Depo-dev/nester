package middleware

import (
	"net/http"
	"strings"

	"github.com/suncrestlabs/nester/apps/api/internal/auth"
)

// WalletBindingCheck returns middleware that verifies the JWT's wallet address
// matches the {address} path parameter on wallet-scoped routes. This prevents
// token replay across different wallets — a token minted for wallet A must not
// be usable against wallet B's endpoints.
//
// The middleware is a no-op for requests that don't carry a user context
// (public routes) or whose path doesn't contain a wallet address parameter.
func WalletBindingCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.GetUserFromContext(r.Context())
		if !ok || user.WalletAddress == "" {
			next.ServeHTTP(w, r)
			return
		}

		pathWallet := extractWalletFromPath(r.URL.Path)
		if pathWallet == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.EqualFold(user.WalletAddress, pathWallet) {
			writeMiddlewareError(w, http.StatusForbidden, "token wallet does not match the requested wallet")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractWalletFromPath looks for a Stellar address (starting with G) in the
// URL path. Returns "" if none is found.
func extractWalletFromPath(path string) string {
	for _, seg := range strings.Split(path, "/") {
		if len(seg) == 56 && strings.HasPrefix(seg, "G") {
			return seg
		}
	}
	return ""
}
