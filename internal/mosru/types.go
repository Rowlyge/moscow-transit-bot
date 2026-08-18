package mosru

import "encoding/json"

// RawRow mirrors one element of a /rows response.
// Cells is kept raw so callers can unmarshal it into their own
// dataset-specific struct (routes, trips, calendar, stop_times all
// have different Cells shapes).
type RawRow struct {
GlobalID int64           `json:"global_id"`
Number   int             `json:"Number"`
Cells    json.RawMessage `json:"Cells"`
}

// FeatureCollection mirrors a /features response (GeoJSON-like).
type FeatureCollection struct {
Type     string    `json:"type"`
Features []Feature `json:"features"`
}

type Feature struct {
Type       string          `json:"type"`
Geometry   Geometry        `json:"geometry"`
Properties FeatureProperties `json:"properties"`
}

type Geometry struct {
Type        string    `json:"type"`
Coordinates []float64 `json:"coordinates"` // [lon, lat]
}

type FeatureProperties struct {
DatasetID  int             `json:"datasetId"`
Attributes json.RawMessage `json:"attributes"`
}
