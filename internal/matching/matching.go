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
RouteShortName string
TripHeadsign   *string
ArrivalTime    string // HH:MM:SS, as stored (can exceed 24:00:00)
}

// StopResult groups arrivals under the stop they belong to. Arrivals is
// empty when the stop exists but has no scheduled stop_times in the
// source data (a known gap in some data.mos.ru records) or none left
// today after the current time. DistanceMeters is nil when the stop
// wasn't found via a geo query (e.g. a name search).
type StopResult struct {
StopID         string
StopName       string
DistanceMeters *float64
Arrivals       []Arrival
}

// activeServiceIDsToday loads today's active calendar service_ids, anchored
// to Moscow local time regardless of the server's system timezone.
func activeServiceIDsToday(ctx context.Context, queries *db.Queries) ([]string, error) {
now := time.Now().In(moscowLocation)

weekday, ok := weekdayNames[now.Weekday()]
if !ok {
return nil, fmt.Errorf("unexpected weekday: %v", now.Weekday())
}

today := pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true}

ids, err := queries.GetActiveServiceIDsToday(ctx, db.GetActiveServiceIDsTodayParams{
Today:   today,
Weekday: weekday,
})
if err != nil {
return nil, fmt.Errorf("loading active service_ids: %w", err)
}
return ids, nil
}

// currentTimeString returns "now" in Moscow local time as HH:MM:SS, for
// comparison against the TEXT-stored arrival_time column.
func currentTimeString() string {
return time.Now().In(moscowLocation).Format("15:04:05")
}

// arrivalsForStop loads upcoming arrivals for a single stop_id, given
// today's active service_ids. Returns an empty (non-nil-checked) slice
// when the stop has nothing scheduled, rather than an error.
func arrivalsForStop(ctx context.Context, queries *db.Queries, stopID string, activeServiceIDs []string, maxArrivals int32) ([]Arrival, error) {
if len(activeServiceIDs) == 0 {
// No active services today (e.g. an unusual holiday calendar gap)
// still yields a valid stop, just with no arrivals.
return nil, nil
}

rows, err := queries.GetUpcomingArrivalsForStop(ctx, db.GetUpcomingArrivalsForStopParams{
StopID:           stopID,
ActiveServiceIds: activeServiceIDs,
CurrentTimeStr:   currentTimeString(),
LimitCount:       maxArrivals,
})
if err != nil {
return nil, fmt.Errorf("loading arrivals for stop %s: %w", stopID, err)
}

arrivals := make([]Arrival, 0, len(rows))
for _, r := range rows {
arrivals = append(arrivals, Arrival{
RouteShortName: r.RouteShortName,
TripHeadsign:   r.TripHeadsign,
ArrivalTime:    r.ArrivalTime,
})
}
return arrivals, nil
}

// NearbyArrivalsParams controls a FindNearbyArrivals call.
type NearbyArrivalsParams struct {
Lat                float64
Lon                float64
MaxStops           int32 // how many nearest stops to consider
MaxArrivalsPerStop int32 // how many upcoming arrivals to return per stop
}

// FindNearbyArrivals implements the geo matching pipeline: geolocation
// -> nearest stops -> today's active service_ids -> upcoming stop_times
// at those stops -> joined route info.
//
// Every stop within MaxStops is included in the result, even if it has
// no upcoming arrivals, so callers can tell the difference between
// "closest stop has nothing scheduled" and "no nearby stops exist".
func FindNearbyArrivals(ctx context.Context, queries *db.Queries, params NearbyArrivalsParams) ([]StopResult, error) {
activeServiceIDs, err := activeServiceIDsToday(ctx, queries)
if err != nil {
return nil, err
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
arrivals, err := arrivalsForStop(ctx, queries, stop.StopID, activeServiceIDs, params.MaxArrivalsPerStop)
if err != nil {
return nil, err
}

distance := stop.DistanceMeters
results = append(results, StopResult{
StopID:         stop.StopID,
StopName:       stop.StopName,
DistanceMeters: &distance,
Arrivals:       arrivals,
})
}

return results, nil
}

// ArrivalsByNameParams controls a FindArrivalsByName call.
type ArrivalsByNameParams struct {
Query              string
MaxStops           int32
MaxArrivalsPerStop int32
}

// FindArrivalsByName implements the name-search matching pipeline:
// stop name substring -> matching stops -> today's active service_ids
// -> upcoming stop_times at those stops -> joined route info.
func FindArrivalsByName(ctx context.Context, queries *db.Queries, params ArrivalsByNameParams) ([]StopResult, error) {
activeServiceIDs, err := activeServiceIDsToday(ctx, queries)
if err != nil {
return nil, err
}

matchedStops, err := queries.SearchStopsByName(ctx, db.SearchStopsByNameParams{
NameQuery:           params.Query,
TransportTypeFilter: "Автобус",
LimitCount:          params.MaxStops,
})
if err != nil {
return nil, fmt.Errorf("searching stops by name: %w", err)
}

results := make([]StopResult, 0, len(matchedStops))
for _, stop := range matchedStops {
arrivals, err := arrivalsForStop(ctx, queries, stop.StopID, activeServiceIDs, params.MaxArrivalsPerStop)
if err != nil {
return nil, err
}

results = append(results, StopResult{
StopID:         stop.StopID,
StopName:       stop.StopName,
DistanceMeters: nil,
Arrivals:       arrivals,
})
}

return results, nil
}

// ErrStopNotFound is returned by ArrivalsForStopID when the given
// stop_id doesn't exist in the stops table.
var ErrStopNotFound = fmt.Errorf("stop not found")

// ArrivalsForStopID looks up a single known stop_id (e.g. a saved
// favorite) and returns its upcoming arrivals, without a geo or name
// search step.
func ArrivalsForStopID(ctx context.Context, queries *db.Queries, stopID string, maxArrivals int32) (StopResult, error) {
stop, err := queries.GetStopByID(ctx, stopID)
if err != nil {
return StopResult{}, fmt.Errorf("%w: %s", ErrStopNotFound, stopID)
}

activeServiceIDs, err := activeServiceIDsToday(ctx, queries)
if err != nil {
return StopResult{}, err
}

arrivals, err := arrivalsForStop(ctx, queries, stopID, activeServiceIDs, maxArrivals)
if err != nil {
return StopResult{}, err
}

return StopResult{
StopID:   stop.StopID,
StopName: stop.StopName,
Arrivals: arrivals,
}, nil
}
