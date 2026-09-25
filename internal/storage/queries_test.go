package storage

import (
	"context"
	"math"
	"os"
	"testing"
)

// newTestDB connects to the shop database from TEST_DSN. The integration
// tests seed their own fixtures and clean them up afterwards, so they run
// both against an empty schema (CI) and a local copy of the stage dump.
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

// Fixture markers: everything the tests create carries one of these,
// so cleanup can never touch real rows.
const (
	fixPaymentID  = "__bot_fixture__"
	fixArticle    = "BTFIX"
	fixCustomer   = "__bot_fixture__"
	fixMethodName = "botfixture"
)

func seedFixtures(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	cleanupFixtures(t, db)

	_, err := db.db.ExecContext(ctx, `
		INSERT INTO cart_order_payment (orderId, paymentId, status, method, price, created_at)
		VALUES (987650001, ?, 'paid',   ?, 100.00, NOW()),
		       (987650002, ?, 'paid',   ?, 100.00, NOW()),
		       (987650003, ?, 'credit', ?,  50.00, NOW())
	`, fixPaymentID+"-1", fixMethodName, fixPaymentID+"-2", fixMethodName, fixPaymentID+"-3", fixMethodName)
	if err != nil {
		t.Fatalf("seed payments: %v", err)
	}

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO catalog_product (template_id, article, type, name, meta_title, meta_description, sold, quantity, visible)
		VALUES (0, 'BTFIX1', 1, '__bot_fixture__ top', '', '', 300, 5, 1),
		       (0, 'BTFIX2', 1, '__bot_fixture__ out', '', '', 200, 0, 1),
		       (0, 'BTFIX3', 1, '__bot_fixture__ low', '', '', 100, 1, 1)
	`)
	if err != nil {
		t.Fatalf("seed products: %v", err)
	}
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO catalog_product_prices (productId, price, priceOld)
		SELECT productId, 99.99, 0 FROM catalog_product WHERE article = 'BTFIX1'
	`)
	if err != nil {
		t.Fatalf("seed product prices: %v", err)
	}

	if _, err := db.db.ExecContext(ctx,
		`INSERT INTO customers (customerName, orderCounter) VALUES (?, 1)`, fixCustomer); err != nil {
		t.Fatalf("seed customers: %v", err)
	}

	t.Cleanup(func() { cleanupFixtures(t, db) })
}

func cleanupFixtures(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	cleanups := []struct {
		query string
		arg   string
	}{
		{`DELETE FROM cart_order_payment WHERE paymentId LIKE ?`, fixPaymentID + "%"},
		{`DELETE FROM catalog_product_prices WHERE productId IN
			(SELECT productId FROM catalog_product WHERE article LIKE ?)`, fixArticle + "%"},
		{`DELETE FROM catalog_product WHERE article LIKE ?`, fixArticle + "%"},
		{`DELETE FROM customers WHERE customerName = ?`, fixCustomer},
	}
	for _, c := range cleanups {
		if _, err := db.db.ExecContext(ctx, c.query, c.arg); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
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
	ctx := context.Background()

	// Start from a state without fixture rows, then take the baseline.
	cleanupFixtures(t, db)
	before, err := db.OrdersStats(ctx, PeriodToday)
	if err != nil {
		t.Fatalf("OrdersStats before: %v", err)
	}

	seedFixtures(t, db)

	after, err := db.OrdersStats(ctx, PeriodToday)
	if err != nil {
		t.Fatalf("OrdersStats: %v", err)
	}

	if after.Total != before.Total+3 {
		t.Errorf("Total = %d, want baseline %d + 3 fixtures", after.Total, before.Total)
	}
	if after.Paid != before.Paid+2 || after.Credit != before.Credit+1 {
		t.Errorf("Paid/Credit = %d/%d, want baseline+2/+1", after.Paid, after.Credit)
	}
	if math.Abs(after.Revenue-(before.Revenue+250)) > 0.01 {
		t.Errorf("Revenue = %.2f, want baseline + 250", after.Revenue)
	}
	if math.Abs(after.PaidRevenue-(before.PaidRevenue+200)) > 0.01 {
		t.Errorf("PaidRevenue = %.2f, want baseline + 200", after.PaidRevenue)
	}
	if after.Avg <= 0 || after.Avg > after.Revenue {
		t.Errorf("Avg = %v is inconsistent with revenue %v", after.Avg, after.Revenue)
	}

	for _, p := range []Period{Period7d, PeriodMonth, PeriodAll} {
		if _, err := db.OrdersStats(ctx, p); err != nil {
			t.Errorf("OrdersStats(%q): %v", p, err)
		}
	}
}

func TestPaymentMethods(t *testing.T) {
	db := newTestDB(t)
	seedFixtures(t, db)

	methods, err := db.PaymentMethods(context.Background(), PeriodToday, 5)
	if err != nil {
		t.Fatalf("PaymentMethods: %v", err)
	}
	if len(methods) == 0 || len(methods) > 5 {
		t.Fatalf("got %d methods, want 1..5", len(methods))
	}
	for i := 1; i < len(methods); i++ {
		if methods[i].Count > methods[i-1].Count {
			t.Error("methods are not sorted by count desc")
		}
	}
	var found bool
	for _, m := range methods {
		if m.Name == fixMethodName {
			found = true
			if m.Count < 3 {
				t.Errorf("fixture method count = %d, want >= 3", m.Count)
			}
		}
	}
	if !found {
		t.Errorf("fixture method %q not found in %+v", fixMethodName, methods)
	}
}

func TestProducts(t *testing.T) {
	db := newTestDB(t)
	seedFixtures(t, db)
	ctx := context.Background()

	summary, err := db.ProductsSummary(ctx)
	if err != nil {
		t.Fatalf("ProductsSummary: %v", err)
	}
	if summary.Total < 3 || summary.Visible < 3 || summary.OutOfStock < 1 {
		t.Errorf("ProductsSummary = %+v, want fixtures counted", summary)
	}

	top, err := db.TopProducts(ctx, 10)
	if err != nil {
		t.Fatalf("TopProducts: %v", err)
	}
	if len(top) < 3 {
		t.Fatalf("TopProducts got %d rows, want >= 3", len(top))
	}
	if top[0].Article != "BTFIX1" {
		t.Errorf("top[0] = %s, want the fixture with highest sold", top[0].Article)
	}
	for i := 1; i < len(top); i++ {
		if top[i].Sold > top[i-1].Sold {
			t.Errorf("top is not sorted by sold desc at %d", i)
		}
	}
	var priceFound bool
	for _, p := range top {
		if p.Article == "BTFIX1" && p.Price != nil && *p.Price == 99.99 {
			priceFound = true
		}
	}
	if !priceFound {
		t.Error("fixture product price was not joined")
	}

	out, err := db.OutOfStock(ctx, 10)
	if err != nil {
		t.Fatalf("OutOfStock: %v", err)
	}
	var outFound bool
	for _, p := range out {
		if p.Quantity == nil || *p.Quantity != 0 {
			t.Errorf("OutOfStock returned product with stock: %+v", p)
		}
		if p.Article == "BTFIX2" {
			outFound = true
		}
	}
	if !outFound {
		t.Error("fixture out-of-stock product not found")
	}
}

func TestCustomersSummary(t *testing.T) {
	db := newTestDB(t)
	seedFixtures(t, db)

	s, err := db.CustomersSummary(context.Background())
	if err != nil {
		t.Fatalf("CustomersSummary: %v", err)
	}
	if s.Total < 1 || s.NewMonth < 1 {
		t.Errorf("CustomersSummary = %+v, want fixture customer counted", s)
	}
}

func TestRecentOrders(t *testing.T) {
	db := newTestDB(t)
	seedFixtures(t, db)

	orders, err := db.RecentOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("RecentOrders: %v", err)
	}
	if len(orders) < 3 {
		t.Fatalf("got %d orders, want >= 3", len(orders))
	}

	// The fixtures are created with NOW(), so they must be the newest.
	fixIDs := map[int64]bool{987650001: true, 987650002: true, 987650003: true}
	got := map[int64]bool{}
	for _, o := range orders[:3] {
		got[o.OrderID] = true
		if o.WhenText == "" {
			t.Errorf("order %d has empty date", o.OrderID)
		}
	}
	for id := range fixIDs {
		if !got[id] {
			t.Errorf("fixture order %d is not among the latest: %v", id, got)
		}
	}
}
