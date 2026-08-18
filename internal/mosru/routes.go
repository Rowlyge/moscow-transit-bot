package mosru

import (
"encoding/json"
"fmt"
"strconv"
)

// RouteCells mirrors the Cells object for dataset 60664 (routes).
// route_type arrives as a JSON number, not a string, so it's decoded
// separately and converted below.
type RouteCells struct {
RouteID        string `json:"route_id"`
AgencyCode     string `json:"agency_code"`
RouteShortName string `json:"route_short_name"`
RouteLongName  string `json:"route_long_name"`
RouteType      int    `json:"route_type"`
RouteDesc      string `json:"route_desc"`
RouteUnion     string `json:"route_union"`
}

// ParsedRoute is the shape used to feed sqlc's UpsertRoute.
type ParsedRoute struct {
RouteID        string
RouteShortName string
RouteLongName  *string
RouteType      *string
}

// ParseRoutes decodes raw /rows rows for the routes dataset into
// ParsedRoute values ready for upsert.
func ParseRoutes(rows []RawRow) ([]ParsedRoute, error) {
parsed := make([]ParsedRoute, 0, len(rows))

for i, row := range rows {
var cells RouteCells
if err := json.Unmarshal(row.Cells, &cells); err != nil {
return nil, fmt.Errorf("row %d (global_id=%d): unmarshalling cells: %w", i, row.GlobalID, err)
}

if cells.RouteID == "" {
return nil, fmt.Errorf("row %d (global_id=%d): empty route_id", i, row.GlobalID)
}

p := ParsedRoute{
RouteID:        cells.RouteID,
RouteShortName: cells.RouteShortName,
}

if cells.RouteLongName != "" {
v := cells.RouteLongName
p.RouteLongName = &v
}

routeTypeStr := strconv.Itoa(cells.RouteType)
p.RouteType = &routeTypeStr

parsed = append(parsed, p)
}

return parsed, nil
}
