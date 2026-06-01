package kite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"be-stonks/internal/provider"
)

const kiteAPIBase = "https://api.kite.trade"

// ltpAttribute is Kite's alert operand attribute for last traded price.
// VERIFIED against gokiteconnect v4.4.0: alerts_test.go and examples/connect/advanced/connect.go
// both use "LastTradedPrice" as the LHSAttribute string for LTP-based alerts.
const ltpAttribute = "LastTradedPrice"

func (k *Kite) authHeader() string {
	return "token " + k.apiKey + ":" + k.token()
}

// ListAlerts returns all alerts currently configured on the Kite account.
func (k *Kite) ListAlerts(ctx context.Context) ([]provider.Alert, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kiteAPIBase+"/alerts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", k.authHeader())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kite list alerts: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	// Kite wraps responses in {"status":"success","data":[...]}
	// The Alert struct in gokiteconnect has json tags: "uuid" and "name".
	var out struct {
		Data []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("kite list alerts: decode: %w", err)
	}
	alerts := make([]provider.Alert, 0, len(out.Data))
	for _, a := range out.Data {
		alerts = append(alerts, provider.Alert{Name: a.Name, UUID: a.UUID})
	}
	return alerts, nil
}

// CreateAlert creates a single simple LTP alert on Kite.
// Param names verified against gokiteconnect v4.4.0 alerts.go CreateAlert:
//   - lhs_exchange, lhs_tradingsymbol, lhs_attribute, operator, rhs_type, rhs_constant
//
// Endpoint: POST /alerts (URIAlerts = "/alerts" in gokiteconnect).
func (k *Kite) CreateAlert(ctx context.Context, spec provider.AlertSpec) error {
	form := url.Values{}
	form.Set("name", spec.Name)
	form.Set("type", "simple")
	form.Set("lhs_exchange", string(spec.Exchange))
	form.Set("lhs_tradingsymbol", spec.Symbol)
	form.Set("lhs_attribute", ltpAttribute)
	form.Set("operator", string(spec.Operator))
	form.Set("rhs_type", "constant")
	form.Set("rhs_constant", strconv.FormatFloat(spec.Value, 'f', -1, 64))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kiteAPIBase+"/alerts", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", k.authHeader())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kite create alert %q: %s: %s", spec.Name, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
