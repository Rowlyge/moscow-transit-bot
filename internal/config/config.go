package config

import (
"fmt"
"os"

"github.com/joho/godotenv"
)

type Config struct {
DatabaseURL      string
MosAPIKey        string
TelegramBotToken string
Socks5Proxy      string // optional: "host:port", empty means no proxy
}

func Load() (*Config, error) {
// .env is optional in production (real env vars may be set directly),
// so we ignore a missing-file error but not other read errors.
if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
return nil, fmt.Errorf("loading .env: %w", err)
}

cfg := &Config{
DatabaseURL:      os.Getenv("DATABASE_URL"),
MosAPIKey:        os.Getenv("MOS_API_KEY"),
TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
Socks5Proxy:      os.Getenv("SOCKS5_PROXY"),
}

if cfg.DatabaseURL == "" {
return nil, fmt.Errorf("DATABASE_URL is not set")
}
if cfg.MosAPIKey == "" {
return nil, fmt.Errorf("MOS_API_KEY is not set")
}
if cfg.TelegramBotToken == "" {
return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
}

return cfg, nil
}
