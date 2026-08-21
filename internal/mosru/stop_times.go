package mosru

import (
"encoding/json"
"fmt"
"strconv"
)

// StopTimeCells mirrors the Cells object for dataset 60661 (stop_times).
// stop_id arrives as a JSON number, not a string. stop_times_id is
// consistently empty in the source data (see migration 000002), so we
// key rows by the row's global_id instead.
type StopTimeCells struct {
StopTimesID   string `json:"stop_times_id"`
TripID        string `json:"trip_id"`
ArrivalTime   string `json:"arrival_time"`
DepartureTime string `json:"departure_time"`
StopID        int64  `json:"stop_id"`
StopSequence  int    `json:"stop_sequence"`
}

// ParsedStopTime is the shape used to feed sqlc's UpsertStopTime.
type ParsedStopTime struct {
GlobalID      int64
StopTimesRaw  string
TripID        string
StopID        string
ArrivalTime   string
DepartureTime string
StopSequence  int32
}

func ParseStopTimes(rows []RawRow) ([]ParsedStopTime, error) {
parsed := make([]ParsedStopTime, 0, len(rows))

for i, row := range rows {
var cells StopTimeCells
if err := json.Unmarshal(row.Cells, &cells); err != nil {
return nil, fmt.Errorf("row %d (global_id=%d): unmarshalling cells: %w", i, row.GlobalID, err)
}

if cells.TripID == "" {
return nil, fmt.Errorf("row %d (global_id=%d): empty trip_id", i, row.GlobalID)
}
if cells.StopID == 0 {
return nil, fmt.Errorf("row %d (global_id=%d): empty stop_id", i, row.GlobalID)
}

parsed = append(parsed, ParsedStopTime{
GlobalID:      row.GlobalID,
StopTimesRaw:  cells.StopTimesID,
TripID:        cells.TripID,
StopID:        strconv.FormatInt(cells.StopID, 10),
ArrivalTime:   cells.ArrivalTime,
DepartureTime: cells.DepartureTime,
StopSequence:  int32(cells.StopSequence),
})
}

return parsed, nil
}
