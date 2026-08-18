package mosru

import (
"encoding/json"
"fmt"
)

// TripCells mirrors the Cells object for dataset 60665 (trips).
type TripCells struct {
RouteID      string `json:"route_id"`
ServiceID    string `json:"service_id"`
TripID       string `json:"trip_id"`
TripHeadsign string `json:"trip_headsign"`
DirectionID  int    `json:"direction_id"`
BlockID      string `json:"block_id"`
VolumeID     string `json:"volume_id"`
TripType     string `json:"trip_type"`
}

// ParsedTrip is the shape used to feed sqlc's UpsertTrip.
type ParsedTrip struct {
TripID       string
RouteID      string
ServiceID    string
TripHeadsign *string
DirectionID  *int16
}

func ParseTrips(rows []RawRow) ([]ParsedTrip, error) {
parsed := make([]ParsedTrip, 0, len(rows))

for i, row := range rows {
var cells TripCells
if err := json.Unmarshal(row.Cells, &cells); err != nil {
return nil, fmt.Errorf("row %d (global_id=%d): unmarshalling cells: %w", i, row.GlobalID, err)
}

if cells.TripID == "" {
return nil, fmt.Errorf("row %d (global_id=%d): empty trip_id", i, row.GlobalID)
}
if cells.RouteID == "" {
return nil, fmt.Errorf("row %d (trip_id=%s): empty route_id", i, cells.TripID)
}
if cells.ServiceID == "" {
return nil, fmt.Errorf("row %d (trip_id=%s): empty service_id", i, cells.TripID)
}

p := ParsedTrip{
TripID:    cells.TripID,
RouteID:   cells.RouteID,
ServiceID: cells.ServiceID,
}

if cells.TripHeadsign != "" {
v := cells.TripHeadsign
p.TripHeadsign = &v
}

directionID := int16(cells.DirectionID)
p.DirectionID = &directionID

parsed = append(parsed, p)
}

return parsed, nil
}
