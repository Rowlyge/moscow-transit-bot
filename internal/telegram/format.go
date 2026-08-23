package telegram

import (
"fmt"
"strings"
"time"

"github.com/Rowlyge/moscow-transit-bot/internal/matching"
)

// FormatArrivals renders a list of stops with their upcoming arrivals as
// a Telegram message. Stops with no upcoming arrivals are shown with a
// note rather than omitted, so the person understands why a stop might
// not have anything listed. Distance is only shown when known (geo
// search); name search results omit it.
func FormatArrivals(results []matching.StopResult) string {
if len(results) == 0 {
return "Ничего не найдено."
}

var b strings.Builder
now := time.Now()

for i, stop := range results {
if i > 0 {
b.WriteString("\n")
}

if stop.DistanceMeters != nil {
fmt.Fprintf(&b, "📍 %s (%s)\n", stop.StopName, formatDistance(*stop.DistanceMeters))
} else {
fmt.Fprintf(&b, "📍 %s\n", stop.StopName)
}

if len(stop.Arrivals) == 0 {
b.WriteString("нет данных о рейсах\n")
continue
}

for _, a := range stop.Arrivals {
minutesLeft := minutesUntil(now, a.ArrivalTime)
fmt.Fprintf(&b, "%s — %s\n", a.RouteShortName, formatMinutes(minutesLeft))
}
}

return b.String()
}

func formatDistance(meters float64) string {
if meters < 1000 {
return fmt.Sprintf("%.0fм", meters)
}
return fmt.Sprintf("%.1fкм", meters/1000)
}

// minutesUntil computes minutes from now until the given HH:MM:SS time
// today. Negative or nonsensical results (e.g. from GTFS times exceeding
// 24:00:00 for trips spanning midnight) are clamped to a safe display
// value rather than shown as-is.
func minutesUntil(now time.Time, arrivalTimeStr string) int {
var h, m, s int
if _, err := fmt.Sscanf(arrivalTimeStr, "%d:%d:%d", &h, &m, &s); err != nil {
return 0
}

daysAhead := h / 24
normalizedHour := h % 24

arrival := time.Date(now.Year(), now.Month(), now.Day(), normalizedHour, m, s, 0, now.Location())
arrival = arrival.AddDate(0, 0, daysAhead)

diff := arrival.Sub(now)
minutes := int(diff.Minutes())
if minutes < 0 {
minutes = 0
}
return minutes
}

func formatMinutes(minutes int) string {
if minutes == 0 {
return "прибывает"
}
return fmt.Sprintf("через %d %s", minutes, russianMinutesWord(minutes))
}

// russianMinutesWord returns the correctly declined Russian word for
// "minute(s)" given a count, following standard pluralization rules:
// 1, 21, 31... -> "минуту"; 2-4, 22-24, 32-34... -> "минуты" (except
// 11-14, which always take "минут"); everything else -> "минут".
func russianMinutesWord(n int) string {
lastTwo := n % 100
lastOne := n % 10

if lastTwo >= 11 && lastTwo <= 14 {
return "минут"
}

switch lastOne {
case 1:
return "минуту"
case 2, 3, 4:
return "минуты"
default:
return "минут"
}
}
