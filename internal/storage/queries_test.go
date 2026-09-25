package storage

import (
	"context"
	"os"
	"testing"
)

// newTestDB connects to the shop database from TEST_DSN. The integration
// tests run against a local copy of the stage dump; they are skipped when
// TEST_DSN is not set.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN is not set; skipping integration test")
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPeriodCond(t *testing.T) {
	for _, p := range []Period{PeriodToday, Period7d, PeriodMonth, PeriodAll} {
		if _, err := periodCond(p); err != nil {
			t.Errorf("periodCond(%q): %v", p, err)
		}
	}
	if _, err := periodCond("bogus"); err == nil {
		t.Error("periodCond(bogus) must fail")
	}
}

func TestOrdersStats(t *testing.T) {
	db := newTestDB(t)

	s, err := db.OrdersStats(context.Background(), PeriodAll)
	if err != nil {
		t.Fatalf("OrdersStats: %v", err)
	}
	if s.Total <= 0 || s.Revenue <= 0 {
		t.Errorf("OrdersStats = %+v, want non-empty totals", s)
	}
	if s.Paid+s.Credit > s.Total {
		t.Errorf("paid+credit (%d+%d) exceeds total %d", s.Paid, s.Credit, s.Total)
	}
	if s.Avg <= 0 || s.Avg > s.Revenue {
		t.Errorf("Avg = %v is inconsistent with revenue %v", s.Avg, s.Revenue)
	}

	for _, p := range []Period{PeriodToday, Period7d, PeriodMonth} {
		if _, err := db.OrdersStats(context.Background(), p); err != nil {
			t.Errorf("OrdersStats(%q): %v", p, err)
		}
	}
}

func TestPaymentMethods(t *testing.T) {
	db := newTestDB(t)

	methods, err := db.PaymentMethods(context.Background(), PeriodAll, 5)
	if err != nil {
		t.Fatalf("PaymentMethods: %v", err)
	}
	if len(methods) == 0 || len(methods) > 5 {
		t.Errorf("got %d methods, want 1..5", len(methods))
	}
	if methods[0].Count < methods[len(methods)-1].Count {
		t.Error("methods are not sorted by count desc")
	}
}

func TestProducts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	summary, err := db.ProductsSummary(ctx)
	if err != nil {
		t.Fatalf("ProductsSummary: %v", err)
	}
	if summary.Total <= 0 || summary.Visible > summary.Total {
		t.Errorf("ProductsSummary = %+v", summary)
	}

	top, err := db.TopProducts(ctx, 10)
	if err != nil {
		t.Fatalf("TopProducts: %v", err)
	}
	if len(top) != 10 {
		t.Fatalf("TopProducts got %d rows, want 10", len(top))
	}
	for i, p := range top {
		if p.Article == "" || p.Name == "" {
			t.Errorf("top[%d] has empty article/name: %+v", i, p)
		}
		if i > 0 && p.Sold > top[i-1].Sold {
			t.Errorf("top is not sorted by sold desc at %d", i)
		}
	}

	out, err := db.OutOfStock(ctx, 10)
	if err != nil {
		t.Fatalf("OutOfStock: %v", err)
	}
	for _, p := range out {
		if p.Quantity == nil || *p.Quantity != 0 {
			t.Errorf("OutOfStock returned product with stock: %+v", p)
		}
	}
}

func TestCustomersSummary(t *testing.T) {
	db := newTestDB(t)

	s, err := db.CustomersSummary(context.Background())
	if err != nil {
		t.Fatalf("CustomersSummary: %v", err)
	}
	if s.Total <= 0 || s.NewMonth > s.Total {
		t.Errorf("CustomersSummary = %+v", s)
	}
}

func TestRecentOrders(t *testing.T) {
	db := newTestDB(t)

	orders, err := db.RecentOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("RecentOrders: %v", err)
	}
	if len(orders) != 10 {
		t.Fatalf("got %d orders, want 10", len(orders))
	}
	for _, o := range orders {
		if o.OrderID == 0 || o.WhenText == "" {
			t.Errorf("order with empty id/date: %+v", o)
		}
	}
}
