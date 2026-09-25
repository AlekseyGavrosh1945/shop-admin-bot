// Command bot runs the Toflow shop statistics Telegram bot.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/config"
	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/server"
	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/storage"
	"github.com/AlekseyGavrosh1945/shop-admin-bot/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("BOT_TOKEN is empty: create a bot via @BotFather and put the token into .env")
	}
	if len(cfg.AdminChatIDs) == 0 {
		log.Warn("ADMIN_CHAT_IDS is empty: nobody can use the bot yet; send /id to the bot and add your chat id")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer db.Close()
	log.Info("database connected")

	bot, err := telegram.New(cfg.BotToken, db, cfg.AdminChatIDs, log)
	if err != nil {
		return fmt.Errorf("init telegram bot: %w", err)
	}

	api := server.New(cfg.HTTPAddr, db, log)
	go func() {
		if err := api.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "err", err)
			stop()
		}
	}()
	log.Info("http server listening", "addr", cfg.HTTPAddr)

	go bot.Start()
	log.Info("telegram bot started", "admins", len(cfg.AdminChatIDs))

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := api.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "err", err)
	}
	bot.Stop()
	return nil
}
