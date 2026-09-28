// Package config loads the bot configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime parameters of the bot.
type Config struct {
	// BotToken is the Telegram bot token from @BotFather.
	BotToken string
	// AdminChatIDs is the whitelist of Telegram chat ids allowed to use the bot.
	AdminChatIDs []int64
	// DatabaseURL is the MySQL DSN of the shop database.
	DatabaseURL string
	// HTTPAddr is the address of the HTTP server serving /healthz.
	HTTPAddr string
	// PollInterval is how often event sources are checked for pushes.
	PollInterval time.Duration
	// StateFile keeps poller cursors between restarts.
	StateFile string
}

// Load reads the configuration from the environment, applying defaults
// for unset variables.
func Load() (Config, error) {
	cfg := Config{
		BotToken:    os.Getenv("BOT_TOKEN"),
		DatabaseURL: getenv("DATABASE_URL", "root:devroot@tcp(127.0.0.1:3307)/toflow?parseTime=true"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8081"),
		StateFile:   getenv("STATE_FILE", "bot_state.json"),
	}

	var err error
	if cfg.PollInterval, err = getDuration("POLL_INTERVAL", 30*time.Second); err != nil {
		return cfg, err
	}

	ids, err := chatIDs(os.Getenv("ADMIN_CHAT_IDS"))
	if err != nil {
		return cfg, err
	}
	cfg.AdminChatIDs = ids
	return cfg, nil
}

// chatIDs parses a comma-separated list of Telegram chat ids.
func chatIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("config: ADMIN_CHAT_IDS: bad chat id %q", p)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", key, err)
	}
	return v, nil
}
