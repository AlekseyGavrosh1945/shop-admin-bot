package telegram

import (
	"fmt"
	"strings"

	tg "gopkg.in/telebot.v3"

	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/storage"
)

// Callback data namespaces: "menu:<view>" and "orders:<period>".
const (
	cbMenu   = "menu"
	cbOrders = "orders"
)

// Views of the main menu.
const (
	viewOrders    = "orders"
	viewProducts  = "products"
	viewCustomers = "customers"
	viewRecent    = "recent"
	viewChats     = "chats"
	viewTickets   = "tickets"
)

func mainMenuMarkup() *tg.ReplyMarkup {
	m := &tg.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("📊 Заказы", cbMenu, viewOrders), m.Data("🛒 Товары", cbMenu, viewProducts)),
		m.Row(m.Data("💬 Чаты", cbMenu, viewChats), m.Data("🎫 Тикеты", cbMenu, viewTickets)),
		m.Row(m.Data("👥 Клиенты", cbMenu, viewCustomers), m.Data("🧾 Последние заказы", cbMenu, viewRecent)),
	)
	return m
}

func mainMenuText() string {
	return "🏪 <b>Toflow</b> — статистика магазина\n\nВыбери раздел:"
}

// periodButtons builds the period switcher row, marking the active period.
func periodButtons(m *tg.ReplyMarkup, active storage.Period) []tg.Btn {
	labels := map[storage.Period]string{
		storage.PeriodToday: "Сегодня",
		storage.Period7d:    "7 дней",
		storage.PeriodMonth: "Месяц",
		storage.PeriodAll:   "Всё время",
	}
	order := []storage.Period{storage.PeriodToday, storage.Period7d, storage.PeriodMonth, storage.PeriodAll}
	btns := make([]tg.Btn, 0, len(order))
	for _, p := range order {
		label := labels[p]
		if p == active {
			label = "• " + label
		}
		btns = append(btns, m.Data(label, cbOrders, string(p)))
	}
	return btns
}

func backRow(m *tg.ReplyMarkup) tg.Row { return m.Row(m.Data("⬅️ Меню", cbMenu, "")) }

// renderOrders builds the orders view: text plus markup for a period.
func renderOrders(s *storage.OrdersStats, methods []storage.NameCount, p storage.Period) (string, *tg.ReplyMarkup) {
	m := &tg.ReplyMarkup{}
	m.Inline(m.Row(periodButtons(m, p)...), backRow(m))

	var b strings.Builder
	fmt.Fprintf(&b, "📊 <b>Заказы</b> — %s\n\n", periodTitle(p))
	fmt.Fprintf(&b, "Всего заказов: <b>%s</b>\n", fmtInt(s.Total))
	fmt.Fprintf(&b, "✅ Оплачено: <b>%s</b> на %s\n", fmtInt(s.Paid), fmtEUR(s.PaidRevenue))
	fmt.Fprintf(&b, "🧾 В кредит: <b>%s</b> на %s\n", fmtInt(s.Credit), fmtEUR(s.Revenue-s.PaidRevenue))
	fmt.Fprintf(&b, "💰 Оборот: <b>%s</b>\n", fmtEUR(s.Revenue))
	fmt.Fprintf(&b, "📈 Средний чек: <b>%s</b>\n", fmtEUR(s.Avg))

	if len(methods) > 0 {
		b.WriteString("\nСпособы оплаты:\n")
		for _, mc := range methods {
			fmt.Fprintf(&b, "• %s — %s\n", esc(mc.Name), fmtInt(mc.Count))
		}
	}
	return b.String(), m
}

// renderProducts builds the products view.
func renderProducts(s *storage.ProductsSummary, top, out []storage.Product) (string, *tg.ReplyMarkup) {
	m := &tg.ReplyMarkup{}
	m.Inline(backRow(m))

	var b strings.Builder
	b.WriteString("🛒 <b>Товары</b>\n\n")
	fmt.Fprintf(&b, "Всего: <b>%s</b>, в витрине: <b>%s</b>\n", fmtInt(s.Total), fmtInt(s.Visible))
	fmt.Fprintf(&b, "Нет в наличии (в витрине): <b>%s</b>\n", fmtInt(s.OutOfStock))

	if len(top) > 0 {
		b.WriteString("\n🔥 Топ продаж:\n")
		for i, p := range top {
			fmt.Fprintf(&b, "%d. %s — продано <b>%s</b>, остаток %s, %s\n",
				i+1, esc(productLabel(p)), fmtInt(p.Sold), fmtQty(p.Quantity), fmtPrice(p.Price))
		}
	}
	if len(out) > 0 {
		b.WriteString("\n⚠️ Популярное, но закончилось:\n")
		for i, p := range out {
			if i >= 5 {
				b.WriteString("…\n")
				break
			}
			fmt.Fprintf(&b, "• %s (продано <b>%s</b>)\n", esc(productLabel(p)), fmtInt(p.Sold))
		}
	}
	return b.String(), m
}

// renderCustomers builds the customers view.
func renderCustomers(s *storage.CustomersSummary) (string, *tg.ReplyMarkup) {
	m := &tg.ReplyMarkup{}
	m.Inline(backRow(m))

	var b strings.Builder
	b.WriteString("👥 <b>Клиенты</b>\n\n")
	fmt.Fprintf(&b, "Всего: <b>%s</b>\n", fmtInt(s.Total))
	fmt.Fprintf(&b, "Новых в этом месяце: <b>%s</b>\n", fmtInt(s.NewMonth))
	return b.String(), m
}

// renderRecent builds the latest orders view.
func renderRecent(orders []storage.RecentOrder) (string, *tg.ReplyMarkup) {
	m := &tg.ReplyMarkup{}
	m.Inline(backRow(m))

	var b strings.Builder
	b.WriteString("🧾 <b>Последние заказы</b>\n\n")
	if len(orders) == 0 {
		b.WriteString("Пока пусто.")
		return b.String(), m
	}
	for _, o := range orders {
		fmt.Fprintf(&b, "<code>#%d</code> %s <b>%s</b> · %s · %s · <i>%s</i>\n",
			o.OrderID, statusEmoji(o.Status), fmtEUR(o.Price), esc(o.Method), esc(o.Status), esc(o.WhenText))
	}
	return b.String(), m
}

// renderChats builds the open chats view.
func renderChats(chats []storage.ChatRow) (string, *tg.ReplyMarkup) {
	m := &tg.ReplyMarkup{}
	m.Inline(backRow(m))

	var b strings.Builder
	if len(chats) == 0 {
		b.WriteString("💬 <b>Чаты</b>\n\nОткрытых чатов нет.")
		return b.String(), m
	}

	unreadTotal := int64(0)
	for _, c := range chats {
		unreadTotal += c.Unread
	}
	fmt.Fprintf(&b, "💬 <b>Открытые чаты</b>: %d, непрочитанных сообщений: <b>%d</b>\n\n", len(chats), unreadTotal)
	for _, c := range chats {
		fmt.Fprintf(&b, "<code>#%d</code> %s (%s)", c.ID, esc(c.Username), esc(c.Email))
		if c.Cause != 0 {
			fmt.Fprintf(&b, " — %s", esc(causeName(c.Cause)))
		}
		if c.OrderID != "" {
			fmt.Fprintf(&b, " · заказ <code>%s</code>", esc(c.OrderID))
		}
		if c.Unread > 0 {
			fmt.Fprintf(&b, " · 🔴 %s", fmtInt(c.Unread))
		}
		b.WriteString("\n")
	}
	return b.String(), m
}

// renderTickets builds the tickets view.
func renderTickets(open, waiting int64, tickets []storage.TicketRow) (string, *tg.ReplyMarkup) {
	m := &tg.ReplyMarkup{}
	m.Inline(backRow(m))

	var b strings.Builder
	fmt.Fprintf(&b, "🎫 <b>Тикеты</b> — открытых: <b>%s</b>, в ожидании: <b>%s</b>\n", fmtInt(open), fmtInt(waiting))
	if len(tickets) == 0 {
		b.WriteString("\nАктивных тикетов нет.")
		return b.String(), m
	}

	b.WriteString("\n")
	for _, t := range tickets {
		fmt.Fprintf(&b, "<code>#%d</code> заказ <code>%s</code> — %s", t.ID, esc(t.OrderID), esc(ticketStatusName(t.Status)))
		if d := strings.TrimSpace(t.Description); d != "" {
			fmt.Fprintf(&b, "\n%s", esc(d))
		}
		b.WriteString("\n\n")
	}
	return b.String(), m
}

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

func ticketStatusName(status int64) string {
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

func statusEmoji(status string) string {
	switch status {
	case "paid":
		return "✅"
	case "credit":
		return "🧾"
	default:
		return "➖"
	}
}

func periodTitle(p storage.Period) string {
	switch p {
	case storage.PeriodToday:
		return "сегодня"
	case storage.Period7d:
		return "последние 7 дней"
	case storage.PeriodMonth:
		return "текущий месяц"
	case storage.PeriodAll:
		return "за всё время"
	default:
		return string(p)
	}
}

func productLabel(p storage.Product) string {
	label := p.Name
	if runes := []rune(label); len(runes) > 36 {
		label = string(runes[:35]) + "…"
	}
	return label
}
