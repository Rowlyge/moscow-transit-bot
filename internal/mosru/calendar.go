package mosru

import (
"encoding/json"
"fmt"
"time"

"github.com/jackc/pgx/v5/pgtype"
)

// CalendarCells mirrors the Cells object for dataset 60666 (calendar).
type CalendarCells struct {
ServiceID string `json:"service_id"`
Monday    int    `json:"monday"`
Tuesday   int    `json:"tuesday"`
Wednesday int    `json:"wednesday"`
Thursday  int    `json:"thursday"`
Friday    int    `json:"friday"`
Saturday  int    `json:"saturday"`
Sunday    int    `json:"sunday"`
StartDate string `json:"start_date"` // format: YYYYMMDD
EndDate   string `json:"end_date"`   // format: YYYYMMDD
}

// ParsedCalendar is the shape used to feed sqlc's UpsertCalendar.
type ParsedCalendar struct {
ServiceID string
Monday    bool
Tuesday   bool
Wednesday bool
Thursday  bool
Friday    bool
Saturday  bool
Sunday    bool
StartDate pgtype.Date
EndDate   pgtype.Date
}

const mosDateLayout = "20060102"

func ParseCalendar(rows []RawRow) ([]ParsedCalendar, error) {
parsed := make([]ParsedCalendar, 0, len(rows))

for i, row := range rows {
var cells CalendarCells
if err := json.Unmarshal(row.Cells, &cells); err != nil {
return nil, fmt.Errorf("row %d (global_id=%d): unmarshalling cells: %w", i, row.GlobalID, err)
}

if cells.ServiceID == "" {
return nil, fmt.Errorf("row %d (global_id=%d): empty service_id", i, row.GlobalID)
}

startDate, err := time.Parse(mosDateLayout, cells.StartDate)
if err != nil {
return nil, fmt.Errorf("row %d (service_id=%s): parsing start_date %q: %w", i, cells.ServiceID, cells.StartDate, err)
}

endDate, err := time.Parse(mosDateLayout, cells.EndDate)
if err != nil {
return nil, fmt.Errorf("row %d (service_id=%s): parsing end_date %q: %w", i, cells.ServiceID, cells.EndDate, err)
}

parsed = append(parsed, ParsedCalendar{
ServiceID: cells.ServiceID,
Monday:    cells.Monday == 1,
Tuesday:   cells.Tuesday == 1,
Wednesday: cells.Wednesday == 1,
Thursday:  cells.Thursday == 1,
Friday:    cells.Friday == 1,
Saturday:  cells.Saturday == 1,
Sunday:    cells.Sunday == 1,
StartDate: pgtype.Date{Time: startDate, Valid: true},
EndDate:   pgtype.Date{Time: endDate, Valid: true},
})
}

return parsed, nil
}
