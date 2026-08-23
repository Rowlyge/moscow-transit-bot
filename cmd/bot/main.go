package main

import (
"context"
"log"
"net/http"

tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
"github.com/jackc/pgx/v5/pgxpool"

"github.com/Rowlyge/moscow-transit-bot/internal/config"
db "github.com/Rowlyge/moscow-transit-bot/internal/db"
"github.com/Rowlyge/moscow-transit-bot/internal/matching"
"github.com/Rowlyge/moscow-transit-bot/internal/telegram"
)

const (
maxNearbyStops     = 3
maxArrivalsPerStop = 3
)

func main() {
cfg, err := config.Load()
if err != nil {
log.Fatalf("loading config: %v", err)
}

ctx := context.Background()
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
if err != nil {
log.Fatalf("connecting to database: %v", err)
}
defer pool.Close()

queries := db.New(pool)

var httpClient *http.Client
if cfg.Socks5Proxy != "" {
httpClient, err = telegram.NewHTTPClient(cfg.Socks5Proxy)
if err != nil {
log.Fatalf("setting up proxy client: %v", err)
}
log.Printf("routing Telegram traffic through SOCKS5 proxy %s", cfg.Socks5Proxy)
}

var bot *tgbotapi.BotAPI
if httpClient != nil {
bot, err = tgbotapi.NewBotAPIWithClient(cfg.TelegramBotToken, tgbotapi.APIEndpoint, httpClient)
} else {
bot, err = tgbotapi.NewBotAPI(cfg.TelegramBotToken)
}
if err != nil {
log.Fatalf("creating bot: %v", err)
}
log.Printf("authorized as @%s", bot.Self.UserName)

updateConfig := tgbotapi.NewUpdate(0)
updateConfig.Timeout = 30
updates := bot.GetUpdatesChan(updateConfig)

for update := range updates {
if update.Message == nil {
continue
}
handleMessage(ctx, bot, queries, update.Message)
}
}

func handleMessage(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
switch {
case msg.Location != nil:
handleLocation(ctx, bot, queries, msg, msg.Location.Latitude, msg.Location.Longitude)
case msg.Command() == "start":
handleStart(bot, msg)
case msg.Text != "":
handleText(ctx, bot, queries, msg)
default:
handleUnknown(bot, msg)
}
}

func handleStart(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
text := "Привет! Отправь геолокацию, и я покажу ближайшие автобусы.\n\n" +
"Если геолокация не отправляется (например, в Telegram Desktop) — напиши координаты текстом: 55.566212, 37.406062"

locationBtn := tgbotapi.NewKeyboardButtonLocation("📍 Отправить геопозицию")
keyboard := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(locationBtn))

reply := tgbotapi.NewMessage(msg.Chat.ID, text)
reply.ReplyMarkup = keyboard
if _, err := bot.Send(reply); err != nil {
log.Printf("sending start reply: %v", err)
}
}

// handleText tries to parse the message as a "lat, lon" coordinate pair
// (a fallback for clients like Telegram Desktop where sending live
// location isn't always available), and falls through to the unknown-
// message handler if it doesn't look like coordinates.
func handleText(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
lat, lon, ok := telegram.ParseCoordinates(msg.Text)
if !ok {
handleUnknown(bot, msg)
return
}

handleLocation(ctx, bot, queries, msg, lat, lon)
}

func handleLocation(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message, lat, lon float64) {
results, err := matching.FindNearbyArrivals(ctx, queries, matching.NearbyArrivalsParams{
Lat:                lat,
Lon:                lon,
MaxStops:           maxNearbyStops,
MaxArrivalsPerStop: maxArrivalsPerStop,
})
if err != nil {
log.Printf("matching failed for chat %d: %v", msg.Chat.ID, err)
reply := tgbotapi.NewMessage(msg.Chat.ID, "Не получилось найти автобусы поблизости, попробуй ещё раз.")
bot.Send(reply)
return
}

text := telegram.FormatArrivals(results)
reply := tgbotapi.NewMessage(msg.Chat.ID, text)
if _, err := bot.Send(reply); err != nil {
log.Printf("sending location reply: %v", err)
}
}

func handleUnknown(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
reply := tgbotapi.NewMessage(msg.Chat.ID, "Отправь геолокацию, чтобы увидеть ближайшие автобусы 🚌\n\nИли напиши координаты текстом: 55.566212, 37.406062")
bot.Send(reply)
}
