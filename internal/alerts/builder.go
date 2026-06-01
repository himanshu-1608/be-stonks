package alerts

import (
	"strconv"

	"be-stonks/internal/alerts/recommendation"
	"be-stonks/internal/provider"
)

// SpecWithTier pairs an alert spec with its human tier label (for the log).
type SpecWithTier struct {
	Spec provider.AlertSpec
	Tier string
}

// Build produces the 3 alert specs for one recommendation:
// Target 1, the exact Midpoint of Target 1 and 2, and Target 2.
// Names are human-readable, underscore-free, and made unique by the value,
// e.g. "TARIL Target 1 305", "TARIL Midpoint 320.5", "TARIL Target 2 336".
func Build(rec recommendation.Recommendation) []SpecWithTier {
	mid := (rec.Target1 + rec.Target2) / 2
	tiers := []struct {
		label string
		value float64
	}{
		{"Target 1", rec.Target1},
		{"Midpoint", mid},
		{"Target 2", rec.Target2},
	}

	out := make([]SpecWithTier, 0, 3)
	for _, t := range tiers {
		name := rec.Symbol + " " + t.label + " " + trim(t.value)
		out = append(out, SpecWithTier{
			Tier: t.label,
			Spec: provider.AlertSpec{
				Name:      name,
				Symbol:    rec.Symbol,
				Exchange:  provider.ExchangeNSE,
				Attribute: provider.AttrLTP,
				Operator:  provider.OpGTE,
				Value:     t.value,
			},
		})
	}
	return out
}

// trim formats a price without trailing zeros: 320.5 not 320.50, 305 not 305.0.
func trim(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
