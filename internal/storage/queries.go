// Package storage provides read-only access to the shop database:
// statistics over orders, products and customers.
package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// DB wraps the MySQL connection pool.
type DB struct {
	db *sql.DB
}

// Open creates a connection pool and verifies it with a ping.
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

// Close releases the pool connections.
func (d *DB) Close() error { return d.db.Close() }

// Ping checks database availability.
func (d *DB) Ping(ctx context.Context) error { return d.db.PingContext(ctx) }

// Period is a reporting time window for order statistics.
type Period string

const (
	PeriodToday Period = "today"
	Period7d    Period = "7d"
	PeriodMonth Period = "month" // current calendar month
	PeriodAll   Period = "all"
)

// periodCond maps a Period to the SQL predicate over cart_order_payment.created_at.
// The predicates are enum-fixed, never built from user input.
func periodCond(p Period) (string, error) {
	switch p {
	case PeriodToday:
		return "created_at >= CURDATE()", nil
	case Period7d:
		return "created_at >= NOW() - INTERVAL 7 DAY", nil
	case PeriodMonth:
		return "created_at >= DATE_FORMAT(NOW(), '%Y-%m-01')", nil
	case PeriodAll:
		return "TRUE", nil
	default:
		return "", fmt.Errorf("storage: unknown period %q", p)
	}
}

// OrdersStats is the aggregate picture of orders for a period.
type OrdersStats struct {
	Total       int64
	Paid        int64
	Credit      int64
	Revenue     float64
	PaidRevenue float64
	Avg         float64
}

// OrdersStats returns order aggregates for the period.
func (d *DB) OrdersStats(ctx context.Context, p Period) (*OrdersStats, error) {
	cond, err := periodCond(p)
	if err != nil {
		return nil, err
	}
	var s OrdersStats
	err = d.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*),
			COALESCE(SUM(status = 'paid'), 0),
			COALESCE(SUM(status = 'credit'), 0),
			COALESCE(SUM(price), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN price ELSE 0 END), 0),
			COALESCE(AVG(price), 0)
		FROM cart_order_payment
		WHERE %s
	`, cond)).Scan(&s.Total, &s.Paid, &s.Credit, &s.Revenue, &s.PaidRevenue, &s.Avg)
	return &s, err
}

// NameCount is a generic "value with a counter" row.
type NameCount struct {
	Name  string
	Count int64
}

// PaymentMethods returns the top payment methods for the period.
func (d *DB) PaymentMethods(ctx context.Context, p Period, limit int) ([]NameCount, error) {
	cond, err := periodCond(p)
	if err != nil {
		return nil, err
	}
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(method, ''), '—') AS m, COUNT(*) AS cnt
		FROM cart_order_payment
		WHERE %s
		GROUP BY m
		ORDER BY cnt DESC
		LIMIT %d
	`, cond, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectNameCount(rows)
}

// Product is a catalog product with sales and stock figures.
type Product struct {
	Article  string
	Name     string
	Sold     int64
	Quantity *int64
	Price    *float64
}

const productColumns = `
	p.article, p.name, p.sold, p.quantity, pr.price
`

const productJoins = `
	FROM catalog_product p
	LEFT JOIN catalog_product_prices pr ON pr.productId = p.productId
`

// TopProducts returns the best selling products.
func (d *DB) TopProducts(ctx context.Context, limit int) ([]Product, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s %s
		WHERE p.sold > 0
		ORDER BY p.sold DESC
		LIMIT %d
	`, productColumns, productJoins, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectProducts(rows)
}

// OutOfStock returns visible products with zero stock, most sold first.
func (d *DB) OutOfStock(ctx context.Context, limit int) ([]Product, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s %s
		WHERE p.visible = 1 AND p.quantity = 0
		ORDER BY p.sold DESC
		LIMIT %d
	`, productColumns, productJoins, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectProducts(rows)
}

// ProductsSummary is the aggregate picture of the catalog.
type ProductsSummary struct {
	Total      int64
	Visible    int64
	OutOfStock int64
}

// ProductsSummary returns catalog-wide counters.
func (d *DB) ProductsSummary(ctx context.Context) (*ProductsSummary, error) {
	var s ProductsSummary
	err := d.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(visible = 1), 0),
			COALESCE(SUM(visible = 1 AND quantity = 0), 0)
		FROM catalog_product
	`).Scan(&s.Total, &s.Visible, &s.OutOfStock)
	return &s, err
}

// CustomersSummary is the aggregate picture of customers.
type CustomersSummary struct {
	Total    int64
	NewMonth int64
}

// CustomersSummary returns customer-wide counters.
func (d *DB) CustomersSummary(ctx context.Context) (*CustomersSummary, error) {
	var s CustomersSummary
	err := d.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(created_at >= DATE_FORMAT(NOW(), '%Y-%m-01')), 0)
		FROM customers
	`).Scan(&s.Total, &s.NewMonth)
	return &s, err
}

// RecentOrder is a single latest order.
type RecentOrder struct {
	OrderID  int64
	Status   string
	Method   string
	Price    float64
	WhenText string
}

// RecentOrders returns the latest orders, newest first.
func (d *DB) RecentOrders(ctx context.Context, limit int) ([]RecentOrder, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			orderId,
			COALESCE(NULLIF(status, ''), '—'),
			COALESCE(NULLIF(method, ''), '—'),
			COALESCE(price, 0),
			DATE_FORMAT(created_at, '%%d.%%m.%%Y %%H:%%i')
		FROM cart_order_payment
		ORDER BY created_at DESC
		LIMIT %d
	`, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []RecentOrder{}
	for rows.Next() {
		var o RecentOrder
		if err := rows.Scan(&o.OrderID, &o.Status, &o.Method, &o.Price, &o.WhenText); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func collectNameCount(rows *sql.Rows) ([]NameCount, error) {
	out := []NameCount{}
	for rows.Next() {
		var nc NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
}

func collectProducts(rows *sql.Rows) ([]Product, error) {
	out := []Product{}
	for rows.Next() {
		var p Product
		var qty sql.NullInt64
		var price sql.NullFloat64
		if err := rows.Scan(&p.Article, &p.Name, &p.Sold, &qty, &price); err != nil {
			return nil, err
		}
		if qty.Valid {
			v := qty.Int64
			p.Quantity = &v
		}
		if price.Valid {
			v := price.Float64
			p.Price = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
