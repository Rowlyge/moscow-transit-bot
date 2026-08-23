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

favoriteCallbackPrefix    = "fav:"
subscribeCallbackPrefix   = "sub:"
unsubscribeCallbackPrefix = "unsub:"
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
case msg.Command() == "subscriptions":
handleSubscriptions(ctx, bot, queries, msg)
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
"Нажми ⭐, чтобы сохранить остановку — дальше просто пиши /next.\n" +
"Нажми 🔔 под конкретным рейсом, чтобы подписаться на маршрут — список подписок в /subscriptions."

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

func handleSubscriptions(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, msg *tgbotapi.Message) {
subs, err := queries.ListSubscriptionsForUser(ctx, msg.From.ID)
if err != nil {
log.Printf("listing subscriptions failed for chat %d: %v", msg.Chat.ID, err)
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Не получилось загрузить подписки."))
return
}

if len(subs) == 0 {
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "У тебя нет подписок. Найди остановку и нажми 🔔 под нужным рейсом."))
return
}

results, err := matching.ArrivalsForSubscriptions(ctx, queries, subs, maxArrivalsPerStop)
if err != nil {
log.Printf("loading subscription arrivals failed for chat %d: %v", msg.Chat.ID, err)
bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Не получилось загрузить рейсы по подпискам."))
return
}

send(bot, msg.Chat.ID, telegram.FormatSubscriptions(results))

// One "unsubscribe" button per subscription, so the person can prune
// individual entries without retyping the search flow.
var rows [][]tgbotapi.InlineKeyboardButton
for _, sub := range subs {
label := fmt.Sprintf("❌ %s — %s", sub.RouteShortName, sub.StopName)
data := fmt.Sprintf("%s%s:%s", unsubscribeCallbackPrefix, sub.StopID, sub.RouteID)
rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(label, data)))
}
keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
unsubMsg := tgbotapi.NewMessage(msg.Chat.ID, "Отменить подписку:")
unsubMsg.ReplyMarkup = keyboard
if _, err := bot.Send(unsubMsg); err != nil {
log.Printf("sending unsubscribe buttons: %v", err)
}
}

// handleCallback dispatches inline button taps by their data prefix:
// "⭐ save favorite", "🔔 subscribe to a route", "❌ unsubscribe".
func handleCallback(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, cb *tgbotapi.CallbackQuery) {
switch {
case strings.HasPrefix(cb.Data, favoriteCallbackPrefix):
handleFavoriteCallback(ctx, bot, queries, cb)
case strings.HasPrefix(cb.Data, subscribeCallbackPrefix):
handleSubscribeCallback(ctx, bot, queries, cb)
case strings.HasPrefix(cb.Data, unsubscribeCallbackPrefix):
handleUnsubscribeCallback(ctx, bot, queries, cb)
}
}

func handleFavoriteCallback(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, cb *tgbotapi.CallbackQuery) {
stopID := strings.TrimPrefix(cb.Data, favoriteCallbackPrefix)

err := queries.UpsertFavorite(ctx, db.UpsertFavoriteParams{
TelegramUserID: cb.From.ID,
StopID:         stopID,
})

answer := tgbotapi.NewCallback(cb.ID, "")
if err != nil {
log.Printf("saving favorite for user %d: %v", cb.From.ID, err)
answer.Text = "Не получилось сохранить, попробуй ещё раз."
} else {
answer.Text = "Сохранено ⭐ Теперь пиши /next"
}
if _, err := bot.Request(answer); err != nil {
log.Printf("answering callback: %v", err)
}
}

// parseStopRouteData splits "<stop_id>:<route_id>" as used in
// subscribe/unsubscribe callback_data.
func parseStopRouteData(data string) (stopID, routeID string, ok bool) {
parts := strings.SplitN(data, ":", 2)
if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
return "", "", false
}
return parts[0], parts[1], true
}

func handleSubscribeCallback(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, cb *tgbotapi.CallbackQuery) {
stopID, routeID, ok := parseStopRouteData(strings.TrimPrefix(cb.Data, subscribeCallbackPrefix))
answer := tgbotapi.NewCallback(cb.ID, "")
if !ok {
answer.Text = "Что-то пошло не так."
bot.Request(answer)
return
}

err := queries.UpsertSubscription(ctx, db.UpsertSubscriptionParams{
TelegramUserID: cb.From.ID,
StopID:         stopID,
RouteID:        routeID,
})
if err != nil {
log.Printf("saving subscription for user %d: %v", cb.From.ID, err)
answer.Text = "Не получилось подписаться, попробуй ещё раз."
} else {
answer.Text = "Подписка оформлена 🔔 Список — в /subscriptions"
}
if _, err := bot.Request(answer); err != nil {
log.Printf("answering callback: %v", err)
}
}

func handleUnsubscribeCallback(ctx context.Context, bot *tgbotapi.BotAPI, queries *db.Queries, cb *tgbotapi.CallbackQuery) {
stopID, routeID, ok := parseStopRouteData(strings.TrimPrefix(cb.Data, unsubscribeCallbackPrefix))
answer := tgbotapi.NewCallback(cb.ID, "")
if !ok {
answer.Text = "Что-то пошло не так."
bot.Request(answer)
return
}

err := queries.DeleteSubscription(ctx, db.DeleteSubscriptionParams{
TelegramUserID: cb.From.ID,
StopID:         stopID,
RouteID:        routeID,
})
if err != nil {
log.Printf("deleting subscription for user %d: %v", cb.From.ID, err)
answer.Text = "Не получилось отменить подписку."
} else {
answer.Text = "Подписка отменена"
}
if _, err := bot.Request(answer); err != nil {
log.Printf("answering callback: %v", err)
}
}

func handleUnknown(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
send(bot, msg.Chat.ID, "Отправь геолокацию 🚌, координаты (55.566212, 37.406062) или название остановки.\n/next — избранная остановка. /subscriptions — твои подписки на маршруты.")
}

// sendStopResults sends the formatted arrivals text, then a message
// with "⭐ save favorite" buttons (one per stop), then a message with
// "🔔 subscribe" buttons (one per unique stop+route with upcoming
// arrivals) so the person can pick exactly which route to follow.
func sendStopResults(bot *tgbotapi.BotAPI, chatID int64, results []matching.StopResult) {
send(bot, chatID, telegram.FormatArrivals(results))

if len(results) == 0 {
return
}

sendFavoriteButtons(bot, chatID, results)
sendSubscribeButtons(bot, chatID, results)
}

func sendFavoriteButtons(bot *tgbotapi.BotAPI, chatID int64, results []matching.StopResult) {
var rows [][]tgbotapi.InlineKeyboardButton
for i, stop := range results {
label := fmt.Sprintf("⭐ %s", stop.StopName)
if stop.DistanceMeters != nil {
label = fmt.Sprintf("⭐ %s (%s)", stop.StopName, formatShortDistance(*stop.DistanceMeters))
} else if hasDuplicateName(results, stop.StopName) {
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

// sendSubscribeButtons offers one button per unique (stop, route) pair
// that actually has upcoming arrivals — subscribing to a route with no
// data right now wouldn't be useful.
func sendSubscribeButtons(bot *tgbotapi.BotAPI, chatID int64, results []matching.StopResult) {
type stopRoute struct {
stopID, stopName, routeID, routeShortName string
}

seen := make(map[string]bool)
var options []stopRoute
for _, stop := range results {
for _, a := range stop.Arrivals {
key := stop.StopID + ":" + a.RouteID
if seen[key] {
continue
}
seen[key] = true
options = append(options, stopRoute{
stopID:         stop.StopID,
stopName:       stop.StopName,
routeID:        a.RouteID,
routeShortName: a.RouteShortName,
})
}
}

if len(options) == 0 {
return
}

var rows [][]tgbotapi.InlineKeyboardButton
for _, opt := range options {
label := fmt.Sprintf("🔔 %s (%s)", opt.routeShortName, opt.stopName)
data := fmt.Sprintf("%s%s:%s", subscribeCallbackPrefix, opt.stopID, opt.routeID)
rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(label, data)))
}

keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
msg := tgbotapi.NewMessage(chatID, "Подписаться на рейс:")
msg.ReplyMarkup = keyboard
if _, err := bot.Send(msg); err != nil {
log.Printf("sending subscribe buttons: %v", err)
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
