package telegram

import (
	"math"
	"strconv"
	"strings"
)

// fmtEUR renders an amount as "12 345 €" (kopecks are rounded).
func fmtEUR(v float64) string { return fmtInt(int64(math.Round(v))) + " €" }

// fmtInt renders an integer with space thousand separators.
func fmtInt(v int64) string {
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

// fmtQty renders stock; a missing value renders as "—".
func fmtQty(q *int64) string {
	if q == nil {
		return "—"
	}
	return fmtInt(*q)
}

// fmtPrice renders a nullable price; a missing value renders as "—".
func fmtPrice(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmtEUR(*p)
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func esc(s string) string { return htmlEscaper.Replace(s) }
