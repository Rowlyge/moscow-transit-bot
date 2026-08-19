package mosru

import (
"encoding/json"
"fmt"
"strconv"
"strings"
)

// flexibleStringSlice unmarshals a field that the API sometimes sends
// as a plain string and sometimes as an array of strings (observed on
// TransportType for stops served by more than one transport type).
type flexibleStringSlice []string

func (f *flexibleStringSlice) UnmarshalJSON(data []byte) error {
// Try array first.
var arr []string
if err := json.Unmarshal(data, &arr); err == nil {
*f = arr
return nil
}

// Fall back to a single string.
var s string
if err := json.Unmarshal(data, &s); err != nil {
return fmt.Errorf("expected string or array of strings, got %s: %w", string(data), err)
}
if s == "" {
*f = nil
} else {
*f = []string{s}
}
return nil
}

// StopAttributes mirrors properties.attributes for dataset 60662 (stops).
// stop_id arrives as a JSON number, not a string. TransportType can be
// a string or an array of strings depending on the stop.
type StopAttributes struct {
StopID        int64                `json:"stop_id"`
StopName      string               `json:"stop_name"`
StationName   string               `json:"StationName"`
Street        string               `json:"Street"`
TransportType flexibleStringSlice  `json:"TransportType"`
}

// ParsedStop is the shape used to feed sqlc's UpsertStop.
type ParsedStop struct {
StopID        string
StopName      string
StopLat       float64
StopLon       float64
TransportType *string
Street        *string
}

// ParseStops decodes GeoJSON features for the stops dataset into
// ParsedStop values ready for upsert. Coordinates come from
// geometry.coordinates ([lon, lat]); everything else comes from
// properties.attributes.
func ParseStops(features []Feature) ([]ParsedStop, error) {
parsed := make([]ParsedStop, 0, len(features))

for i, f := range features {
var attrs StopAttributes
if err := json.Unmarshal(f.Properties.Attributes, &attrs); err != nil {
return nil, fmt.Errorf("feature %d: unmarshalling attributes: %w", i, err)
}

if attrs.StopID == 0 {
return nil, fmt.Errorf("feature %d: missing stop_id", i)
}

if len(f.Geometry.Coordinates) != 2 {
return nil, fmt.Errorf("feature %d (stop_id=%d): expected 2 coordinates, got %d", i, attrs.StopID, len(f.Geometry.Coordinates))
}

p := ParsedStop{
StopID:   strconv.FormatInt(attrs.StopID, 10),
StopName: attrs.StopName,
StopLon:  f.Geometry.Coordinates[0],
StopLat:  f.Geometry.Coordinates[1],
}

if len(attrs.TransportType) > 0 {
// Multiple transport types are joined into one string
// (e.g. "Автобус, Трамвай") since our schema stores a
// single value; the matching query filters with LIKE
// or ANY(string_to_array(...)) if this needs splitting
// later.
v := strings.Join(attrs.TransportType, ", ")
p.TransportType = &v
}
if attrs.Street != "" {
v := attrs.Street
p.Street = &v
}

parsed = append(parsed, p)
}

return parsed, nil
}
