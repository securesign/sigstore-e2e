package benchmark

import (
	"context"
	"sync"
	"time"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
)

// TokenManager handles shared access and periodic refresh of a token.
type TokenManager struct {
	token string
	mu    sync.RWMutex
}

// NewTokenManager initializes a new TokenManager.
func NewTokenManager(ctx context.Context) *TokenManager {
	manager := &TokenManager{}
	// Start the token refresh in a background goroutine
	manager.RefreshToken(ctx)
	go manager.startTokenRefresher(ctx)
	return manager
}

// GetToken returns the current token value with read lock.
func (tm *TokenManager) GetToken() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.token
}

// RefreshToken refreshes the token value with write lock.
func (tm *TokenManager) RefreshToken(ctx context.Context) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var err error
	tm.token, err = testsupport.GetOIDCToken(ctx, api.GetValueFor(api.OidcIssuerURL), "jdoe", "secure", api.GetValueFor(api.OidcRealm))
	if err != nil {
		logrus.Errorf("failed to get OIDC token %v", err)
	}
	logrus.Info("successfully refreshed token")
}

// startTokenRefresher runs a loop that refreshes the token every 30 seconds.
func (tm *TokenManager) startTokenRefresher(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.RefreshToken(ctx) // Refresh the token every 30 seconds
		case <-ctx.Done():
			// Context is canceled, stop the refresher
			logrus.Info("Stopping token refresher")
			return
		}
	}
}
