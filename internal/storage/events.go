package storage

import (
	"context"
	"fmt"
)

// Event rows for the notification poller. Every source is keyed by an
// auto-increment id, so "since" cursors are simple integer comparisons.

type OrderEvent struct {
	ID       int64
	OrderID  int64
	Status   string
	Method   string
	Price    float64
	WhenText string
}

type ChatMessageEvent struct {
	ID       int64
	ChatID   int64
	Username string
	Email    string
	Cause    int64
	Text     string
}

type TicketEvent struct {
	ID          int64
	OrderID     string
	Status      int64
	Description string
}

type ReviewEvent struct {
	ID        int64
	Author    string
	Rating    int64
	Text      string
	Published bool
}

type RmaEvent struct {
	ID          int64
	OrderID     string
	Description string
}

type RegistrationEvent struct {
	ID    int64
	Email string
}

const maxOrders = `
	SELECT COALESCE(MAX(id), 0) FROM cart_order_payment
`

// OrdersSince returns orders with id greater than since, oldest first.
func (d *DB) OrdersSince(ctx context.Context, since int64, limit int) ([]OrderEvent, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, orderId,
		       COALESCE(NULLIF(status, ''), '—'),
		       COALESCE(NULLIF(method, ''), '—'),
		       COALESCE(price, 0),
		       DATE_FORMAT(created_at, '%%d.%%m %%H:%%i')
		FROM cart_order_payment
		WHERE id > %d
		ORDER BY id
		LIMIT %d
	`, since, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []OrderEvent{}
	for rows.Next() {
		var e OrderEvent
		if err := rows.Scan(&e.ID, &e.OrderID, &e.Status, &e.Method, &e.Price, &e.WhenText); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) OrdersMaxID(ctx context.Context) (int64, error) {
	var id int64
	err := d.db.QueryRowContext(ctx, maxOrders).Scan(&id)
	return id, err
}

// Chat messages: type '100' with support=0 are messages written by
// customers or the site widget; support=1 is an admin reply.
const chatMessageFilter = `type = '100' AND support = 0`

// ChatMessagesSince returns new customer chat messages with chat details.
func (d *DB) ChatMessagesSince(ctx context.Context, since int64, limit int) ([]ChatMessageEvent, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT m.id, m.value,
		       COALESCE(c.username, ''), COALESCE(c.email, ''), COALESCE(c.cause, 0),
		       COALESCE(LEFT(m.message, 200), '')
		FROM message m
		LEFT JOIN chat c ON c.id = m.value
		WHERE m.id > %d AND m.%s
		ORDER BY m.id
		LIMIT %d
	`, since, chatMessageFilter, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ChatMessageEvent{}
	for rows.Next() {
		var e ChatMessageEvent
		if err := rows.Scan(&e.ID, &e.ChatID, &e.Username, &e.Email, &e.Cause, &e.Text); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) ChatMessagesMaxID(ctx context.Context) (int64, error) {
	var id int64
	err := d.db.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT COALESCE(MAX(id), 0) FROM message WHERE %s`, chatMessageFilter)).Scan(&id)
	return id, err
}

// TicketsSince returns newly created tickets.
func (d *DB) TicketsSince(ctx context.Context, since int64, limit int) ([]TicketEvent, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, orderId, status, COALESCE(LEFT(description, 200), '')
		FROM ticket
		WHERE id > %d
		ORDER BY id
		LIMIT %d
	`, since, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TicketEvent{}
	for rows.Next() {
		var e TicketEvent
		if err := rows.Scan(&e.ID, &e.OrderID, &e.Status, &e.Description); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) TicketsMaxID(ctx context.Context) (int64, error) {
	var id int64
	err := d.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM ticket`).Scan(&id)
	return id, err
}

// ReviewsSince returns newly written product reviews.
func (d *DB) ReviewsSince(ctx context.Context, since int64, limit int) ([]ReviewEvent, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(author, ''), COALESCE(rating, 0), COALESCE(text, ''), statusWebshop = 1
		FROM catalog_product_reviews
		WHERE id > %d
		ORDER BY id
		LIMIT %d
	`, since, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ReviewEvent{}
	for rows.Next() {
		var e ReviewEvent
		if err := rows.Scan(&e.ID, &e.Author, &e.Rating, &e.Text, &e.Published); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) ReviewsMaxID(ctx context.Context) (int64, error) {
	var id int64
	err := d.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM catalog_product_reviews`).Scan(&id)
	return id, err
}

// RmaSince returns new RMA (return) requests.
func (d *DB) RmaSince(ctx context.Context, since int64, limit int) ([]RmaEvent, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, orderId, COALESCE(LEFT(description, 200), '')
		FROM rma
		WHERE id > %d
		ORDER BY id
		LIMIT %d
	`, since, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RmaEvent{}
	for rows.Next() {
		var e RmaEvent
		if err := rows.Scan(&e.ID, &e.OrderID, &e.Description); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) RmaMaxID(ctx context.Context) (int64, error) {
	var id int64
	err := d.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM rma`).Scan(&id)
	return id, err
}

// RegistrationsSince returns new registration requests.
func (d *DB) RegistrationsSince(ctx context.Context, since int64, limit int) ([]RegistrationEvent, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, from_mail
		FROM registration_users_by_mail
		WHERE id > %d
		ORDER BY id
		LIMIT %d
	`, since, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RegistrationEvent{}
	for rows.Next() {
		var e RegistrationEvent
		if err := rows.Scan(&e.ID, &e.Email); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) RegistrationsMaxID(ctx context.Context) (int64, error) {
	var id int64
	err := d.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM registration_users_by_mail`).Scan(&id)
	return id, err
}

// ChatRow is an open chat for the "chats" menu view.
type ChatRow struct {
	ID       int64
	Username string
	Email    string
	Cause    int64
	OrderID  string
	Unread   int64
}

// OpenChats returns open chats, newest first, with unread client message counts.
func (d *DB) OpenChats(ctx context.Context, limit int) ([]ChatRow, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT c.id,
		       COALESCE(NULLIF(c.username, ''), '—'),
		       COALESCE(NULLIF(c.email, ''), '—'),
		       COALESCE(c.cause, 0),
		       COALESCE(c.orderId, ''),
		       (SELECT COUNT(*) FROM message m
		        WHERE m.value = c.id AND m.support = 0 AND m.`+"`read`"+` = 0)
		FROM chat c
		WHERE c.status = 1
		ORDER BY c.id DESC
		LIMIT %d
	`, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ChatRow{}
	for rows.Next() {
		var c ChatRow
		if err := rows.Scan(&c.ID, &c.Username, &c.Email, &c.Cause, &c.OrderID, &c.Unread); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TicketRow is an open or waiting ticket for the "tickets" menu view.
type TicketRow struct {
	ID          int64
	OrderID     string
	Status      int64
	Description string
}

// TicketsCounts returns the number of tickets in "open" and "waiting" states.
func (d *DB) TicketsCounts(ctx context.Context) (open, waiting int64, err error) {
	err = d.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(status = 2), 0), COALESCE(SUM(status = 3), 0)
		FROM ticket
	`).Scan(&open, &waiting)
	return open, waiting, err
}

// OpenTickets returns open/waiting tickets, newest first.
func (d *DB) OpenTickets(ctx context.Context, limit int) ([]TicketRow, error) {
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, orderId, status, COALESCE(LEFT(description, 120), '')
		FROM ticket
		WHERE status IN (2, 3)
		ORDER BY id DESC
		LIMIT %d
	`, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TicketRow{}
	for rows.Next() {
		var t TicketRow
		if err := rows.Scan(&t.ID, &t.OrderID, &t.Status, &t.Description); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
