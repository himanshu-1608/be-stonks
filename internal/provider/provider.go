package provider

import "context"

// AlertProvider creates and lists alerts on a broker.
type AlertProvider interface {
	ListAlerts(ctx context.Context) ([]Alert, error)
	CreateAlert(ctx context.Context, spec AlertSpec) error
}

// Authenticator handles a broker's login/session lifecycle.
type Authenticator interface {
	LoginURL() string
	ExchangeToken(requestToken string) error // mints and persists a session
	Ready() bool                             // a session is loaded
}

// Provider is a fully capable broker integration.
// Future capabilities (e.g. TickProvider) get embedded here as they are added.
type Provider interface {
	Name() string
	Authenticator
	AlertProvider
}
