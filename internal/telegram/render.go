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
)

func mainMenuMarkup() *tg.ReplyMarkup {
	m := &tg.ReplyMarkup{}
	m.Inline(
		m.Row(m.Data("📊 Заказы", cbMenu, viewOrders), m.Data("🛒 Товары", cbMenu, viewProducts)),
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
