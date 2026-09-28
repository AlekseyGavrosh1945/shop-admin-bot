// Package ui holds shared text formatting helpers used by the bot UI
// and the notification sources.
package ui

import (
	"math"
	"strconv"
	"strings"
)

// FmtEUR renders an amount as "12 345 €" (cents are rounded).
func FmtEUR(v float64) string { return FmtInt(int64(math.Round(v))) + " €" }

// FmtInt renders an integer with space thousand separators.
func FmtInt(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	digits := strconv.FormatInt(v, 10)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(d)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// FmtQty renders stock; a missing value renders as "—".
func FmtQty(q *int64) string {
	if q == nil {
		return "—"
	}
	return FmtInt(*q)
}

// FmtPrice renders a nullable price; a missing value renders as "—".
func FmtPrice(p *float64) string {
	if p == nil {
		return "—"
	}
	return FmtEUR(*p)
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// Esc escapes text for Telegram HTML parse mode.
func Esc(s string) string { return htmlEscaper.Replace(s) }
