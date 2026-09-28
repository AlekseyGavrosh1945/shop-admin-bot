// Package sources builds poller sources from the shop storage: it maps
// database rows to rendered event texts.
package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/poller"
	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/storage"
	ui "github.com/AlekseyGavrosh1945/shop-admin-bot/internal/ui"
)

// All returns every monitored source.
func All(db *storage.DB) []poller.Source {
	return []poller.Source{
		{
			Name: "orders",
			Fetch: func(ctx context.Context, since int64, limit int) ([]poller.Event, error) {
				rows, err := db.OrdersSince(ctx, since, limit)
				if err != nil {
					return nil, err
				}
				out := make([]poller.Event, 0, len(rows))
				for _, r := range rows {
					out = append(out, poller.Event{ID: r.ID, Text: FormatOrder(r)})
				}
				return out, nil
			},
			MaxID: db.OrdersMaxID,
		},
		{
			Name: "chat_messages",
			Fetch: func(ctx context.Context, since int64, limit int) ([]poller.Event, error) {
				rows, err := db.ChatMessagesSince(ctx, since, limit)
				if err != nil {
					return nil, err
				}
				out := make([]poller.Event, 0, len(rows))
				for _, r := range rows {
					out = append(out, poller.Event{ID: r.ID, Text: FormatChatMessage(r)})
				}
				return out, nil
			},
			MaxID: db.ChatMessagesMaxID,
		},
		{
			Name: "tickets",
			Fetch: func(ctx context.Context, since int64, limit int) ([]poller.Event, error) {
				rows, err := db.TicketsSince(ctx, since, limit)
				if err != nil {
					return nil, err
				}
				out := make([]poller.Event, 0, len(rows))
				for _, r := range rows {
					out = append(out, poller.Event{ID: r.ID, Text: FormatTicket(r)})
				}
				return out, nil
			},
			MaxID: db.TicketsMaxID,
		},
		{
			Name: "reviews",
			Fetch: func(ctx context.Context, since int64, limit int) ([]poller.Event, error) {
				rows, err := db.ReviewsSince(ctx, since, limit)
				if err != nil {
					return nil, err
				}
				out := make([]poller.Event, 0, len(rows))
				for _, r := range rows {
					out = append(out, poller.Event{ID: r.ID, Text: FormatReview(r)})
				}
				return out, nil
			},
			MaxID: db.ReviewsMaxID,
		},
		{
			Name: "rma",
			Fetch: func(ctx context.Context, since int64, limit int) ([]poller.Event, error) {
				rows, err := db.RmaSince(ctx, since, limit)
				if err != nil {
					return nil, err
				}
				out := make([]poller.Event, 0, len(rows))
				for _, r := range rows {
					out = append(out, poller.Event{ID: r.ID, Text: FormatRma(r)})
				}
				return out, nil
			},
			MaxID: db.RmaMaxID,
		},
		{
			Name: "registrations",
			Fetch: func(ctx context.Context, since int64, limit int) ([]poller.Event, error) {
				rows, err := db.RegistrationsSince(ctx, since, limit)
				if err != nil {
					return nil, err
				}
				out := make([]poller.Event, 0, len(rows))
				for _, r := range rows {
					out = append(out, poller.Event{ID: r.ID, Text: FormatRegistration(r)})
				}
				return out, nil
			},
			MaxID: db.RegistrationsMaxID,
		},
	}
}

// FormatOrder renders an order event line.
func FormatOrder(o storage.OrderEvent) string {
	return fmt.Sprintf("💰 Заказ <code>#%d</code> — <b>%s</b> · %s · %s · %s",
		o.OrderID, ui.FmtEUR(o.Price), ui.Esc(o.Status), ui.Esc(o.Method), ui.Esc(o.WhenText))
}

// FormatChatMessage renders a customer chat message line.
func FormatChatMessage(m storage.ChatMessageEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "💬 %s в чате <code>#%d</code>", ui.Esc(chatAuthor(m.Username, m.Email)), m.ChatID)
	if m.Cause != 0 {
		fmt.Fprintf(&b, " (%s)", ui.Esc(causeName(m.Cause)))
	}
	if t := strings.TrimSpace(m.Text); t != "" {
		fmt.Fprintf(&b, ":\n%s", ui.Esc(t))
	}
	return b.String()
}

// FormatTicket renders a new ticket line.
func FormatTicket(t storage.TicketEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🎫 Тикет <code>#%d</code> по заказу <code>%s</code> — %s",
		t.ID, ui.Esc(t.OrderID), ui.Esc(ticketStatus(t.Status)))
	if d := strings.TrimSpace(t.Description); d != "" {
		fmt.Fprintf(&b, ":\n%s", ui.Esc(d))
	}
	return b.String()
}

// FormatReview renders a new product review line.
func FormatReview(r storage.ReviewEvent) string {
	state := "ждёт модерации"
	if r.Published {
		state = "опубликован"
	}
	return fmt.Sprintf("⭐ Отзыв от <b>%s</b>, оценка %d/5 (%s):\n%s",
		ui.Esc(r.Author), r.Rating, state, ui.Esc(r.Text))
}

// FormatRma renders a new RMA request line.
func FormatRma(r storage.RmaEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📦 RMA <code>#%d</code> по заказу <code>%s</code>", r.ID, ui.Esc(r.OrderID))
	if d := strings.TrimSpace(r.Description); d != "" {
		fmt.Fprintf(&b, ":\n%s", ui.Esc(d))
	}
	return b.String()
}

// FormatRegistration renders a new registration request line.
func FormatRegistration(r storage.RegistrationEvent) string {
	return fmt.Sprintf("✍️ Заявка на регистрацию от <b>%s</b>", ui.Esc(r.Email))
}

// BatchText groups event lines under one header.
func BatchText(source string, events []poller.Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s</b>\n", ui.Esc(sourceTitle(source)))
	for _, e := range events {
		b.WriteString(e.Text)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func sourceTitle(source string) string {
	switch source {
	case "orders":
		return "💰 Новые заказы"
	case "chat_messages":
		return "💬 Новые сообщения в чатах"
	case "tickets":
		return "🎫 Новые тикеты"
	case "reviews":
		return "⭐ Новые отзывы"
	case "rma":
		return "📦 Новые RMA-запросы"
	case "registrations":
		return "✍️ Новые заявки на регистрацию"
	default:
		return source
	}
}

func chatAuthor(username, email string) string {
	if username != "" && username != "—" {
		return username
	}
	if email != "" && email != "—" {
		return email
	}
	return "Клиент"
}

// causeName maps the chat.cause values used by Toflow (Dutch labels).
func causeName(cause int64) string {
	names := map[int64]string{
		1: "вопрос по товару",
		2: "запрос цены",
		3: "вопрос по счёту",
		4: "технический вопрос",
		5: "вопрос о наличии",
	}
	if n, ok := names[cause]; ok {
		return n
	}
	return fmt.Sprintf("вопрос #%d", cause)
}

func ticketStatus(status int64) string {
	switch status {
	case 1:
		return "отвечен"
	case 2:
		return "открыт"
	case 3:
		return "в ожидании"
	case 4:
		return "закрыт"
	default:
		return fmt.Sprintf("статус %d", status)
	}
}
