package main

import (
"context"
"fmt"
"log"
"net/http"
"strings"

tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
"github.com/jackc/pgx/v5/pgxpool"

"github.com/Rowlyge/moscow-transit-bot/internal/config"
db "github.com/Rowlyge/moscow-transit-bot/internal/db"
"github.com/Rowlyge/moscow-transit-bot/internal/matching"
"github.com/Rowlyge/moscow-transit-bot/internal/telegram"
)

const (
maxNearbyStops     = 3
maxNameMatches     = 5
maxArrivalsPerStop = 3

favoriteCallbackPrefix = "fav:"
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
if update.CallbackQuery != nil {
handleCallback(ctx, bot, queries, update.CallbackQuery)
continue
}
if update.Message == nil {
continue
}
handleMessage(ctx, bot, queries, update.Message)
}
}

func handleMessage(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
switch {
case msg.Location != nil:
handleGeoLocation(ctx, bot, queries, msg, msg.Location.Latitude, msg.Location.Longitude)
case msg.Command() == "start":
handleStart(bot, msg)
case msg.Command() == "next":
handleNext(ctx, bot, queries, msg)
case msg.Text != "":
handleText(ctx, bot, queries, msg)
default:
handleUnknown(bot, msg)
}
}

func handleStart(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
text := "Привет! Отправь геолокацию, и я покажу ближайшие автобусы.\n\n" +
"Также можно:\n" +
"— написать координаты текстом: 55.566212, 37.406062\n" +
"— написать название остановки: Зимёнковская улица\n\n" +
"Нажми ⭐ под остановкой, чтобы сохранить её как избранную — дальше просто пиши /next."

locationBtn := tgbotapi.NewKeyboardButtonLocation("📍 Отправить геопозицию")
keyboard := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(locationBtn))

reply := tgbotapi.NewMessage(msg.Chat.ID, text)
reply.ReplyMarkup = keyboard
if _, err := bot.Send(reply); err != nil {
log.Printf("sending start reply: %v", err)
}
}

// handleText tries the message as coordinates first (a fallback for
// clients like Telegram Desktop where sending live location isn't
// always available), then as a stop name search, and finally falls
// through to the unknown-message handler.
func handleText(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
if lat, lon, ok := telegram.ParseCoordinates(msg.Text); ok {
handleGeoLocation(ctx, bot, queries, msg, lat, lon)
return
}

handleNameSearch(ctx, bot, queries, msg)
}

func handleGeoLocation(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message, lat, lon float64) {
results, err := matching.FindNearbyArrivals(ctx, queries, matching.NearbyArrivalsParams{
Lat:                lat,
Lon:                lon,
MaxStops:           maxNearbyStops,
MaxArrivalsPerStop: maxArrivalsPerStop,
})
if err != nil {
log.Printf("geo matching failed for chat %d: %v", msg.Chat.ID, err)
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Не получилось найти автобусы поблизости, попробуй ещё раз."))
return
}

sendStopResults(bot, msg.Chat.ID, results)
}

func handleNameSearch(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
results, err := matching.FindArrivalsByName(ctx, queries, matching.ArrivalsByNameParams{
Query:              msg.Text,
MaxStops:           maxNameMatches,
MaxArrivalsPerStop: maxArrivalsPerStop,
})
if err != nil {
log.Printf("name search failed for chat %d: %v", msg.Chat.ID, err)
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Не получилось выполнить поиск, попробуй ещё раз."))
return
}

if len(results) == 0 {
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Остановка не найдена. Проверь название или отправь геолокацию."))
return
}

sendStopResults(bot, msg.Chat.ID, results)
}

func handleNext(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
stopID, err := queries.GetFavorite(ctx, msg.From.ID)
if err != nil {
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "У тебя ещё нет избранной остановки. Найди остановку и нажми ⭐, чтобы сохранить."))
return
}

result, err := matching.ArrivalsForStopID(ctx, queries, stopID, maxArrivalsPerStop)
if err != nil {
log.Printf("loading favorite failed for chat %d: %v", msg.Chat.ID, err)
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Не получилось загрузить избранную остановку."))
return
}

sendStopResults(bot, msg.Chat.ID, []matching.StopResult{result})
}

// handleCallback handles taps on a "⭐ save this stop" button, saving
// that stop as the user's favorite.
func handleCallback(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, cb *tgbotapi.CallbackQuery) {
if !strings.HasPrefix(cb.Data, favoriteCallbackPrefix) {
return
}
stopID := strings.TrimPrefix(cb.Data, favoriteCallbackPrefix)

err := queries.UpsertFavorite(ctx, db.UpsertFavoriteParams{
TelegramUserID: cb.From.ID,
StopID:         stopID,
})

callbackConfig := tgbotapi.NewCallback(cb.ID, "")
if err != nil {
log.Printf("saving favorite for user %d: %v", cb.From.ID, err)
callbackConfig.Text = "Не получилось сохранить, попробуй ещё раз."
} else {
callbackConfig.Text = "Сохранено ⭐ Теперь пиши /next"
}
if _, err := bot.Request(callbackConfig); err != nil {
log.Printf("answering callback: %v", err)
}
}

func handleUnknown(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
send(bot, msg.Chat.ID, "Отправь геолокацию 🚌, координаты (55.566212, 37.406062) или название остановки. /next — избранная остановка.")
}

// sendStopResults sends the formatted arrivals text, then (if there's
// more than one stop, or the single stop has no favorite-worthy
// distinction) a single message with one inline "save as favorite"
// button per stop, so tapping saves that exact stop_id.
func sendStopResults(bot *tgbotapi.BotAPI, chatID int64, results []matching.StopResult) {
send(bot, chatID, telegram.FormatArrivals(results))

if len(results) == 0 {
return
}

var rows [][]tgbotapi.InlineKeyboardButton
for i, stop := range results {
label := fmt.Sprintf("⭐ %s", stop.StopName)
if stop.DistanceMeters != nil {
label = fmt.Sprintf("⭐ %s (%s)", stop.StopName, formatShortDistance(*stop.DistanceMeters))
} else if hasDuplicateName(results, stop.StopName) {
// Disambiguate identically-named stops (e.g. two directions
// of the same physical stop) when there's no distance to show.
label = fmt.Sprintf("⭐ %s #%d", stop.StopName, i+1)
}

btn := tgbotapi.NewInlineKeyboardButtonData(label, favoriteCallbackPrefix+stop.StopID)
rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
}

keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
msg := tgbotapi.NewMessage(chatID, "Сохранить как избранную:")
msg.ReplyMarkup = keyboard
if _, err := bot.Send(msg); err != nil {
log.Printf("sending favorite buttons: %v", err)
}
}

func hasDuplicateName(results []matching.StopResult, name string) bool {
count := 0
for _, r := range results {
if r.StopName == name {
count++
}
}
return count > 1
}

func formatShortDistance(meters float64) string {
if meters < 1000 {
return fmt.Sprintf("%.0fм", meters)
}
return fmt.Sprintf("%.1fкм", meters/1000)
}

func send(bot *tgbotapi.BotAPI, chatID int64, text string) {
if _, err := bot.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
log.Printf("sending message to chat %d: %v", chatID, err)
}
}
