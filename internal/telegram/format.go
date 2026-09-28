// Package telegram renders bot views. This file aliases the shared
// formatting helpers for concise local use.
package telegram

import (
	ui "github.com/AlekseyGavrosh1945/shop-admin-bot/internal/ui"
)

func fmtEUR(v float64) string { return ui.FmtEUR(v) }

func fmtInt(v int64) string { return ui.FmtInt(v) }

func fmtQty(q *int64) string { return ui.FmtQty(q) }

func fmtPrice(p *float64) string { return ui.FmtPrice(p) }

func esc(s string) string { return ui.Esc(s) }
