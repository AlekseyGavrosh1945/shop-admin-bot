package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ADMIN_CHAT_IDS", "")
	t.Setenv("HTTP_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BotToken != "" {
		t.Errorf("BotToken = %q, want empty", cfg.BotToken)
	}
	if cfg.DatabaseURL != "root:devroot@tcp(127.0.0.1:3307)/toflow?parseTime=true" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.HTTPAddr != ":8081" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if len(cfg.AdminChatIDs) != 0 {
		t.Errorf("AdminChatIDs = %v, want empty", cfg.AdminChatIDs)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("BOT_TOKEN", "secret")
	t.Setenv("DATABASE_URL", "u:p@tcp(h:3306)/db")
	t.Setenv("ADMIN_CHAT_IDS", " 111, 222 ,,-333")
	t.Setenv("HTTP_ADDR", ":9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BotToken != "secret" || cfg.DatabaseURL != "u:p@tcp(h:3306)/db" || cfg.HTTPAddr != ":9090" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.AdminChatIDs) != 3 || cfg.AdminChatIDs[0] != 111 || cfg.AdminChatIDs[1] != 222 || cfg.AdminChatIDs[2] != -333 {
		t.Errorf("AdminChatIDs = %v", cfg.AdminChatIDs)
	}
}

func TestLoadRejectsBadChatIDs(t *testing.T) {
	t.Setenv("ADMIN_CHAT_IDS", "111,abc")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a non-numeric chat id")
	}
}
