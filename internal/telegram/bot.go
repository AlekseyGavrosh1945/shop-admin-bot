// Package telegram implements the admin bot: a button menu over
// read-only shop statistics.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tg "gopkg.in/telebot.v3"

	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/storage"
)

// Store is the read-only data surface the bot needs.
type Store interface {
	OrdersStats(ctx context.Context, p storage.Period) (*storage.OrdersStats, error)
	PaymentMethods(ctx context.Context, p storage.Period, limit int) ([]storage.NameCount, error)
	TopProducts(ctx context.Context, limit int) ([]storage.Product, error)
	OutOfStock(ctx context.Context, limit int) ([]storage.Product, error)
	ProductsSummary(ctx context.Context) (*storage.ProductsSummary, error)
	CustomersSummary(ctx context.Context) (*storage.CustomersSummary, error)
	RecentOrders(ctx context.Context, limit int) ([]storage.RecentOrder, error)
}

// Bot wraps the telebot client with an admin whitelist.
type Bot struct {
	bot    *tg.Bot
	store  Store
	admins map[int64]bool
	log    *slog.Logger
}

// New creates the bot; an empty token returns (nil, nil).
func New(token string, store Store, adminIDs []int64, log *slog.Logger) (*Bot, error) {
	if token == "" {
		return nil, nil
	}
	b, err := tg.NewBot(tg.Settings{
		Token:  token,
		Poller: &tg.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	bot := &Bot{
		bot:    b,
		store:  store,
		admins: make(map[int64]bool, len(adminIDs)),
		log:    log,
	}
	for _, id := range adminIDs {
		bot.admins[id] = true
	}

	b.Handle("/start", bot.onMenu)
	b.Handle("/menu", bot.onMenu)
	b.Handle("/id", bot.onID)
	// A single callback router: data is "menu:<view>" or "orders:<period>".
	b.Handle(tg.OnCallback, bot.onCallback)
	return bot, nil
}

// Start runs the long-polling loop; it blocks until Stop is called.
func (b *Bot) Start() { b.bot.Start() }

// Stop stops the polling loop.
func (b *Bot) Stop() { b.bot.Stop() }

// onID reports the caller's chat id so it can be whitelisted in
// ADMIN_CHAT_IDS. Available to everyone on purpose.
func (b *Bot) onID(c tg.Context) error {
	return c.Send(fmt.Sprintf("Твой chat ID: <code>%d</code>\nДобавь его в ADMIN_CHAT_IDS и перезапусти бота.", c.Sender().ID))
}

func (b *Bot) onMenu(c tg.Context) error {
	if !b.allowed(c.Sender().ID) {
		return c.Send("⛔ Нет доступа. Твой chat ID можно узнать командой /id")
	}
	return c.Send(mainMenuText(), mainMenuMarkup())
}

// onCallback routes button presses. telebot strips the "\f<unique>"
// prefix, so the fired button is identified via c.Callback().Unique
// and its payload arrives as c.Callback().Data.
func (b *Bot) onCallback(c tg.Context) error {
	if !b.allowed(c.Sender().ID) {
		return c.Respond(&tg.CallbackResponse{Text: "⛔ Нет доступа"})
	}

	switch cb := c.Callback(); cb.Unique {
	case cbMenu:
		return b.showView(c, cb.Data)
	case cbOrders:
		return b.showOrders(c, storage.Period(cb.Data))
	default:
		return c.Respond(&tg.CallbackResponse{Text: "Неизвестная кнопка"})
	}
}

func (b *Bot) showView(c tg.Context, view string) error {
	ctx := context.Background()
	switch view {
	case viewOrders:
		if err := b.editOrders(c, ctx, storage.PeriodMonth); err != nil {
			b.log.Error("build orders view", "err", err)
			return c.Respond(&tg.CallbackResponse{Text: "Ошибка базы данных, попробуй позже"})
		}
		return c.Respond()
	case viewProducts:
		text, markup, err := b.productsView(ctx)
		if err != nil {
			b.log.Error("build products view", "err", err)
			return c.Respond(&tg.CallbackResponse{Text: "Ошибка базы данных, попробуй позже"})
		}
		return b.edit(c, text, markup)
	case viewCustomers:
		text, markup, err := b.customersView(ctx)
		if err != nil {
			b.log.Error("build customers view", "err", err)
			return c.Respond(&tg.CallbackResponse{Text: "Ошибка базы данных, попробуй позже"})
		}
		return b.edit(c, text, markup)
	case viewRecent:
		text, markup, err := b.recentView(ctx)
		if err != nil {
			b.log.Error("build recent view", "err", err)
			return c.Respond(&tg.CallbackResponse{Text: "Ошибка базы данных, попробуй позже"})
		}
		return b.edit(c, text, markup)
	default:
		return b.edit(c, mainMenuText(), mainMenuMarkup())
	}
}

// edit replaces the message the button was pressed on.
func (b *Bot) edit(c tg.Context, text string, markup *tg.ReplyMarkup) error {
	if err := c.Edit(text, markup, tg.ModeHTML); err != nil {
		b.log.Warn("edit message", "err", err)
	}
	return c.Respond()
}

func (b *Bot) showOrders(c tg.Context, p storage.Period) error {
	if err := b.editOrders(c, context.Background(), p); err != nil {
		b.log.Error("build orders view", "period", p, "err", err)
		return c.Respond(&tg.CallbackResponse{Text: "Ошибка базы данных, попробуй позже"})
	}
	return c.Respond()
}

func (b *Bot) editOrders(c tg.Context, ctx context.Context, p storage.Period) error {
	stats, err := b.store.OrdersStats(ctx, p)
	if err != nil {
		return err
	}
	methods, err := b.store.PaymentMethods(ctx, p, 5)
	if err != nil {
		return err
	}
	text, markup := renderOrders(stats, methods, p)
	if err := c.Edit(text, markup, tg.ModeHTML); err != nil {
		b.log.Warn("edit message", "err", err)
	}
	return nil
}

func (b *Bot) productsView(ctx context.Context) (string, *tg.ReplyMarkup, error) {
	summary, err := b.store.ProductsSummary(ctx)
	if err != nil {
		return "", nil, err
	}
	top, err := b.store.TopProducts(ctx, 10)
	if err != nil {
		return "", nil, err
	}
	out, err := b.store.OutOfStock(ctx, 10)
	if err != nil {
		return "", nil, err
	}
	text, markup := renderProducts(summary, top, out)
	return text, markup, nil
}

func (b *Bot) customersView(ctx context.Context) (string, *tg.ReplyMarkup, error) {
	s, err := b.store.CustomersSummary(ctx)
	if err != nil {
		return "", nil, err
	}
	text, markup := renderCustomers(s)
	return text, markup, nil
}

func (b *Bot) recentView(ctx context.Context) (string, *tg.ReplyMarkup, error) {
	orders, err := b.store.RecentOrders(ctx, 10)
	if err != nil {
		return "", nil, err
	}
	text, markup := renderRecent(orders)
	return text, markup, nil
}

// allowed reports whether the chat is in the admin whitelist.
func (b *Bot) allowed(chatID int64) bool { return b.admins[chatID] }
