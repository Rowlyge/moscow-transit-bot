package telegram

import (
"fmt"
"strconv"
"strings"
)

// ParseCoordinates attempts to parse a message like "55.566212, 37.406062"
// (comma or space separated, with or without spaces) into lat/lon floats.
// Returns ok=false if the text doesn't look like a coordinate pair, so
// callers can fall back to other message handling without treating a
// parse failure as an error.
func ParseCoordinates(text string) (lat, lon float64, ok bool) {
text = strings.TrimSpace(text)
if text == "" {
return 0, 0, false
}

// Accept both "55.5, 37.4" and "55.5 37.4" as separators.
normalized := strings.ReplaceAll(text, ",", " ")
fields := strings.Fields(normalized)
if len(fields) != 2 {
return 0, 0, false
}

parsedLat, err := strconv.ParseFloat(fields[0], 64)
if err != nil {
return 0, 0, false
}
parsedLon, err := strconv.ParseFloat(fields[1], 64)
if err != nil {
return 0, 0, false
}

// Sanity-check the values look like real coordinates, not just any
// two numbers someone happened to type (e.g. "18 25" in a chat).
if parsedLat < -90 || parsedLat > 90 || parsedLon < -180 || parsedLon > 180 {
return 0, 0, false
}

// Loosely scope to the Moscow region to avoid accidentally matching
// unrelated numeric messages as coordinates.
const (
moscowLatMin, moscowLatMax = 54.0, 57.0
moscowLonMin, moscowLonMax = 35.0, 40.0
)
if parsedLat < moscowLatMin || parsedLat > moscowLatMax || parsedLon < moscowLonMin || parsedLon > moscowLonMax {
return 0, 0, false
}

return parsedLat, parsedLon, true
}

// FormatCoordinateHint returns a short explanation of the expected
// coordinate format, shown when parsing fails but the message looked
// like an attempt at coordinates.
func FormatCoordinateHint() string {
return fmt.Sprintf("Не получилось распознать координаты. Пример: 55.566212, 37.406062")
}
