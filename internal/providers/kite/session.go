package kite

import (
	"encoding/json"
	"os"
	"path/filepath"

	"be-stonks/internal/timeutil"
)

type sessionFile struct {
	AccessToken string `json:"access_token"`
	SavedAt     string `json:"saved_at"`
}

// LoginURL returns the Kite hosted login URL.
func (k *Kite) LoginURL() string {
	return k.client.GetLoginURL()
}

// ExchangeToken trades a request_token for an access_token, sets it on the
// client, and persists it.
func (k *Kite) ExchangeToken(requestToken string) error {
	sess, err := k.client.GenerateSession(requestToken, k.apiSecret)
	if err != nil {
		return err
	}
	k.client.SetAccessToken(sess.AccessToken)

	k.mu.Lock()
	k.accessToken = sess.AccessToken
	k.ready = true
	k.mu.Unlock()

	return k.saveSession(sess.AccessToken)
}

// Ready reports whether an access token is loaded.
func (k *Kite) Ready() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.ready
}

func (k *Kite) token() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.accessToken
}

func (k *Kite) saveSession(token string) error {
	if err := os.MkdirAll(filepath.Dir(k.sessionPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sessionFile{
		AccessToken: token,
		SavedAt:     timeutil.Label(timeutil.NowIST()),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(k.sessionPath, b, 0o600)
}

func (k *Kite) loadSession() {
	b, err := os.ReadFile(k.sessionPath)
	if err != nil {
		return
	}
	var sf sessionFile
	if err := json.Unmarshal(b, &sf); err != nil || sf.AccessToken == "" {
		return
	}
	k.client.SetAccessToken(sf.AccessToken)
	k.mu.Lock()
	k.accessToken = sf.AccessToken
	k.ready = true
	k.mu.Unlock()
}
