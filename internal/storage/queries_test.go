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

	// Chat with one unread customer message.
	chatRes, err := db.db.ExecContext(ctx, `
		INSERT INTO chat (uid, time, status, username, email, cause, orderId)
		VALUES (1, UNIX_TIMESTAMP(), 1, ?, 'fixture@example.com', 5, 'BTFIX-ORDER')
	`, fixCustomer)
	if err != nil {
		t.Fatalf("seed chat: %v", err)
	}
	chatID, _ := chatRes.LastInsertId()

	if _, err := db.db.ExecContext(ctx, `
		INSERT INTO message (type, value, uid, support, `+"`read`"+`, message, time)
		VALUES ('100', ?, 0, 0, 0, '__bot_fixture__ question', UNIX_TIMESTAMP())
	`, chatID); err != nil {
		t.Fatalf("seed chat message: %v", err)
	}

	// Ticket in "open" state.
	if _, err := db.db.ExecContext(ctx, `
		INSERT INTO ticket (uid, orderId, cause, department, status, view, time, description)
		VALUES (1, 'BTFIX-ORDER', 4, 1, 2, '0', UNIX_TIMESTAMP(), '__bot_fixture__ ticket')
	`); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	// Review on the fixture product (created above with article BTFIX1).
	if _, err := db.db.ExecContext(ctx, `
		INSERT INTO catalog_product_reviews (productId, author, email, rating, statusWebshop, text)
		SELECT productId, ?, 'review@example.com', 4, 0, '__bot_fixture__ review'
		FROM catalog_product WHERE article = 'BTFIX1'
	`, fixCustomer); err != nil {
		t.Fatalf("seed review: %v", err)
	}

	// RMA request.
	if _, err := db.db.ExecContext(ctx, `
		INSERT INTO rma (uid, orderId, description, isSubscribe, replacement, status, time)
		VALUES (1, 'BTFIX-ORDER', '__bot_fixture__ rma', 0, 0, 1, UNIX_TIMESTAMP())
	`); err != nil {
		t.Fatalf("seed rma: %v", err)
	}

	// Registration request.
	if _, err := db.db.ExecContext(ctx, `
		INSERT INTO registration_users_by_mail (id_sender, from_mail, key_url, status, time)
		VALUES (0, 'fixture@example.com', 'fixkey', 0, UNIX_TIMESTAMP())
	`); err != nil {
		t.Fatalf("seed registration: %v", err)
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
		{`DELETE FROM message WHERE message LIKE ?`, fixPaymentID + "%"},
		{`DELETE FROM chat WHERE username = ?`, fixCustomer},
		{`DELETE FROM ticket WHERE description LIKE ?`, fixPaymentID + "%"},
		{`DELETE FROM catalog_product_reviews WHERE author = ?`, fixCustomer},
		{`DELETE FROM rma WHERE description LIKE ?`, fixPaymentID + "%"},
		{`DELETE FROM registration_users_by_mail WHERE from_mail = ?`, "fixture@example.com"},
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

func TestEventSources(t *testing.T) {
	db := newTestDB(t)
	seedFixtures(t, db)
	ctx := context.Background()

	t.Run("ChatMessages", func(t *testing.T) {
		// A production cursor sits just behind the newest row, so the
		// window after MAX(id)-1 must contain exactly the fixture.
		maxID, err := db.ChatMessagesMaxID(ctx)
		if err != nil {
			t.Fatalf("ChatMessagesMaxID: %v", err)
		}
		msgs, err := db.ChatMessagesSince(ctx, maxID-1, 20)
		if err != nil {
			t.Fatalf("ChatMessagesSince: %v", err)
		}
		if len(msgs) != 1 {
			t.Fatalf("got %d messages after maxID-1, want exactly the fixture", len(msgs))
		}
		m := msgs[0]
		if m.Text != "__bot_fixture__ question" {
			t.Errorf("message = %+v, want the fixture", m)
		}
		if m.Username != fixCustomer || m.Cause != 5 || m.ChatID == 0 {
			t.Errorf("fixture chat message = %+v", m)
		}
		if m.ID != maxID {
			t.Errorf("message id = %d, want maxID %d", m.ID, maxID)
		}
	})

	t.Run("Tickets", func(t *testing.T) {
		maxID, err := db.TicketsMaxID(ctx)
		if err != nil {
			t.Fatalf("TicketsMaxID: %v", err)
		}
		tickets, err := db.TicketsSince(ctx, maxID-1, 20)
		if err != nil {
			t.Fatalf("TicketsSince: %v", err)
		}
		var found bool
		for _, tk := range tickets {
			if tk.Description == "__bot_fixture__ ticket" {
				found = true
				if tk.Status != 2 || tk.OrderID != "BTFIX-ORDER" {
					t.Errorf("fixture ticket = %+v", tk)
				}
			}
		}
		if !found {
			t.Error("fixture ticket not found")
		}
	})

	t.Run("Reviews", func(t *testing.T) {
		reviews, err := db.ReviewsSince(ctx, 0, 20)
		if err != nil {
			t.Fatalf("ReviewsSince: %v", err)
		}
		var found bool
		for _, r := range reviews {
			if r.Text == "__bot_fixture__ review" {
				found = true
				if r.Author != fixCustomer || r.Rating != 4 || r.Published {
					t.Errorf("fixture review = %+v", r)
				}
			}
		}
		if !found {
			t.Error("fixture review not found")
		}
	})

	t.Run("Rma", func(t *testing.T) {
		maxID, err := db.RmaMaxID(ctx)
		if err != nil {
			t.Fatalf("RmaMaxID: %v", err)
		}
		rmas, err := db.RmaSince(ctx, maxID-1, 20)
		if err != nil {
			t.Fatalf("RmaSince: %v", err)
		}
		var found bool
		for _, r := range rmas {
			if r.Description == "__bot_fixture__ rma" {
				found = true
			}
		}
		if !found {
			t.Error("fixture rma not found")
		}
		if _, err := db.RmaMaxID(ctx); err != nil {
			t.Errorf("RmaMaxID: %v", err)
		}
	})

	t.Run("Registrations", func(t *testing.T) {
		regs, err := db.RegistrationsSince(ctx, 0, 20)
		if err != nil {
			t.Fatalf("RegistrationsSince: %v", err)
		}
		var found bool
		for _, r := range regs {
			if r.Email == "fixture@example.com" {
				found = true
			}
		}
		if !found {
			t.Error("fixture registration not found")
		}
		if _, err := db.RegistrationsMaxID(ctx); err != nil {
			t.Errorf("RegistrationsMaxID: %v", err)
		}
	})

	t.Run("OrdersSince", func(t *testing.T) {
		orders, err := db.OrdersSince(ctx, 0, 20)
		if err != nil {
			t.Fatalf("OrdersSince: %v", err)
		}
		if len(orders) == 0 {
			t.Fatal("OrdersSince(0) returned no rows")
		}
		for i := 1; i < len(orders); i++ {
			if orders[i].ID <= orders[i-1].ID {
				t.Error("OrdersSince is not sorted by id asc")
			}
		}
	})
}

func TestOpenChatsAndTickets(t *testing.T) {
	db := newTestDB(t)
	seedFixtures(t, db)
	ctx := context.Background()

	chats, err := db.OpenChats(ctx, 10)
	if err != nil {
		t.Fatalf("OpenChats: %v", err)
	}
	var chatFound bool
	for _, c := range chats {
		if c.Username == fixCustomer {
			chatFound = true
			if c.Cause != 5 || c.OrderID != "BTFIX-ORDER" {
				t.Errorf("fixture chat = %+v", c)
			}
			if c.Unread != 1 {
				t.Errorf("fixture chat unread = %d, want 1", c.Unread)
			}
		}
	}
	if !chatFound {
		t.Error("fixture chat not found among open chats")
	}

	open, _, err := db.TicketsCounts(ctx)
	if err != nil {
		t.Fatalf("TicketsCounts: %v", err)
	}
	if open < 1 {
		t.Errorf("open tickets = %d, want >= 1", open)
	}

	tickets, err := db.OpenTickets(ctx, 10)
	if err != nil {
		t.Fatalf("OpenTickets: %v", err)
	}
	var ticketFound bool
	for _, tk := range tickets {
		if tk.Description == "__bot_fixture__ ticket" {
			ticketFound = true
			if tk.Status != 2 {
				t.Errorf("fixture ticket status = %d, want 2", tk.Status)
			}
		}
	}
	if !ticketFound {
		t.Error("fixture ticket not found among open tickets")
	}
}
