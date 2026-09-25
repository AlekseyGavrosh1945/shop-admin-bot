package telegram

import (
	"strings"
	"testing"

	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/storage"
)

func TestRenderOrders(t *testing.T) {
	stats := &storage.OrdersStats{
		Total: 14, Paid: 10, Credit: 4,
		Revenue: 17234.5, PaidRevenue: 12000, Avg: 1231.04,
	}
	methods := []storage.NameCount{
		{Name: "ideal", Count: 6},
		{Name: "", Count: 8},
	}

	text, markup := renderOrders(stats, methods, storage.PeriodMonth)

	for _, want := range []string{"<b>Заказы</b>", "текущий месяц", "12 000", "17 235", "1 231", "ideal"} {
		if !strings.Contains(text, want) {
			t.Errorf("renderOrders text missing %q\n%s", want, text)
		}
	}
	if markup == nil || len(markup.InlineKeyboard) < 2 {
		t.Fatalf("markup must contain period row and back row, got %+v", markup)
	}
	if len(markup.InlineKeyboard[0]) != 4 {
		t.Errorf("period row must have 4 buttons, got %d", len(markup.InlineKeyboard[0]))
	}
	if first := markup.InlineKeyboard[0][2].Text; first != "• Месяц" {
		t.Errorf("active period button = %q, want %q", first, "• Месяц")
	}
}

func TestRenderProducts(t *testing.T) {
	qty := int64(0)
	price := 85.0
	summary := &storage.ProductsSummary{Total: 3797, Visible: 3348, OutOfStock: 512}
	top := []storage.Product{
		{Article: "1140574", Name: `HGST 600GB SAS 15K 12Gbps 2,5" SFF`, Sold: 100, Quantity: &qty, Price: &price},
	}
	out := []storage.Product{
		{Article: "8898340", Name: "HP 560SFP+ Adapter", Sold: 100},
	}

	text, _ := renderProducts(summary, top, out)

	for _, want := range []string{
		"<b>Товары</b>", "3 797", "3 348", "512",
		`HGST 600GB SAS 15K 12Gbps 2,5" SFF`, "продано <b>100</b>", "85 €",
		"закончилось", "HP 560SFP+ Adapter",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("renderProducts text missing %q\n%s", want, text)
		}
	}
}

func TestRenderRecentEmpty(t *testing.T) {
	text, markup := renderRecent(nil)
	if !strings.Contains(text, "Пока пусто") {
		t.Errorf("renderRecent empty text = %q", text)
	}
	if markup == nil {
		t.Fatal("markup must not be nil")
	}
}

func TestRenderCustomers(t *testing.T) {
	text, _ := renderCustomers(&storage.CustomersSummary{Total: 9498, NewMonth: 23})
	for _, want := range []string{"9 498", "23"} {
		if !strings.Contains(text, want) {
			t.Errorf("renderCustomers missing %q", want)
		}
	}
}

func TestProductLabelTruncates(t *testing.T) {
	long := strings.Repeat("а", 100)
	got := productLabel(storage.Product{Name: long})
	if runes := len([]rune(got)); runes != 36 {
		t.Errorf("productLabel length = %d runes, want 36", runes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("productLabel must end with ellipsis, got %q", got)
	}
}
