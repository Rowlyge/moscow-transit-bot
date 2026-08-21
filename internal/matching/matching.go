package matching

import (
"context"
"fmt"
"time"

"github.com/jackc/pgx/v5/pgtype"

db "github.com/Rowlyge/moscow-transit-bot/internal/db"
)

// Moscow buses run on Moscow local time regardless of where the server
// hosting the bot lives, so we anchor "today" and "now" to this location
// rather than the server's system timezone.
var moscowLocation = mustLoadLocation("Europe/Moscow")

func mustLoadLocation(name string) *time.Location {
loc, err := time.LoadLocation(name)
if err != nil {
// Fallback: fixed UTC+3 offset, since Moscow does not observe DST.
return time.FixedZone("MSK", 3*60*60)
}
return loc
}

var weekdayNames = map[time.Weekday]string{
time.Monday:    "monday",
time.Tuesday:   "tuesday",
time.Wednesday: "wednesday",
time.Thursday:  "thursday",
time.Friday:    "friday",
time.Saturday:  "saturday",
time.Sunday:    "sunday",
}

// Arrival is a single upcoming bus arrival at a specific stop, ready to
// display to the user.
type Arrival struct {
StopID         string
StopName       string
DistanceMeters float64
RouteShortName string
TripHeadsign   *string
ArrivalTime    string // HH:MM:SS, as stored (can exceed 24:00:00)
}

// StopResult groups arrivals under the stop they belong to. Arrivals is
// empty when the stop exists nearby but has no scheduled stop_times in
// the source data (a known gap in some data.mos.ru records) or none
// left today after the current time.
type StopResult struct {
StopID         string
StopName       string
DistanceMeters float64
Arrivals       []Arrival
}

// NearbyArrivalsParams controls a FindNearbyArrivals call.
type NearbyArrivalsParams struct {
Lat                float64
Lon                float64
MaxStops           int32 // how many nearest stops to consider
MaxArrivalsPerStop int32 // how many upcoming arrivals to return per stop
}

// FindNearbyArrivals implements the core matching pipeline:
// geolocation -> nearest stops -> today's active service_ids ->
// upcoming stop_times at those stops -> joined route info.
//
// Every stop within MaxStops is included in the result, even if it has
// no upcoming arrivals, so callers can tell the difference between
// "closest stop has nothing scheduled" and "no nearby stops exist".
func FindNearbyArrivals(ctx context.Context, queries *db.Queries, params NearbyArrivalsParams) ([]StopResult, error) {
now := time.Now().In(moscowLocation)

weekday, ok := weekdayNames[now.Weekday()]
if !ok {
return nil, fmt.Errorf("unexpected weekday: %v", now.Weekday())
}

today := pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
currentTimeStr := now.Format("15:04:05")

activeServiceIDs, err := queries.GetActiveServiceIDsToday(ctx, db.GetActiveServiceIDsTodayParams{
Today:   today,
Weekday: weekday,
})
if err != nil {
return nil, fmt.Errorf("loading active service_ids: %w", err)
}

nearestStops, err := queries.FindNearestStops(ctx, db.FindNearestStopsParams{
Lat:                 params.Lat,
Lon:                 params.Lon,
TransportTypeFilter: "Автобус",
LimitCount:          params.MaxStops,
})
if err != nil {
return nil, fmt.Errorf("finding nearest stops: %w", err)
}

results := make([]StopResult, 0, len(nearestStops))
for _, stop := range nearestStops {
result := StopResult{
StopID:         stop.StopID,
StopName:       stop.StopName,
DistanceMeters: stop.DistanceMeters,
}

// No active services today (e.g. an unusual holiday calendar gap)
// still yields a valid stop list, just with no arrivals anywhere.
if len(activeServiceIDs) > 0 {
arrivals, err := queries.GetUpcomingArrivalsForStop(ctx, db.GetUpcomingArrivalsForStopParams{
StopID:           stop.StopID,
ActiveServiceIds: activeServiceIDs,
CurrentTimeStr:   currentTimeStr,
LimitCount:       params.MaxArrivalsPerStop,
})
if err != nil {
return nil, fmt.Errorf("loading arrivals for stop %s: %w", stop.StopID, err)
}

for _, a := range arrivals {
result.Arrivals = append(result.Arrivals, Arrival{
StopID:         stop.StopID,
StopName:       stop.StopName,
DistanceMeters: stop.DistanceMeters,
RouteShortName: a.RouteShortName,
TripHeadsign:   a.TripHeadsign,
ArrivalTime:    a.ArrivalTime,
})
}
}

results = append(results, result)
}

return results, nil
}
