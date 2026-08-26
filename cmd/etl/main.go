package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Rowlyge/moscow-transit-bot/internal/config"
	db "github.com/Rowlyge/moscow-transit-bot/internal/db"
	"github.com/Rowlyge/moscow-transit-bot/internal/mosru"
)

const (
	datasetIDRoutes    = 60664
	datasetIDCalendar  = 60666
	datasetIDTrips     = 60665
	datasetIDStops     = 60662
	datasetIDStopTimes = 60661
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
	if cfg.MosAPIKey == "" {
		log.Fatal("MOS_API_KEY is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	client := mosru.NewClient(cfg.MosAPIKey)

	if err := syncRoutes(ctx, client, queries); err != nil {
		log.Fatalf("syncing routes: %v", err)
	}

	if err := syncCalendar(ctx, client, queries); err != nil {
		log.Fatalf("syncing calendar: %v", err)
	}

	if err := syncTrips(ctx, client, pool, queries); err != nil {
		log.Fatalf("syncing trips: %v", err)
	}

	if err := syncStops(ctx, client, queries); err != nil {
		log.Fatalf("syncing stops: %v", err)
	}

	if err := syncStopTimes(ctx, client, pool); err != nil {
		log.Fatalf("syncing stop_times: %v", err)
	}

	log.Println("ETL run complete")
}

func syncRoutes(ctx context.Context, client *mosru.Client, queries *db.Queries) error {
	log.Println("fetching routes...")

	rows, err := client.FetchRows(datasetIDRoutes)
	if err != nil {
		return err
	}
	log.Printf("fetched %d raw rows", len(rows))

	parsed, err := mosru.ParseRoutes(rows)
	if err != nil {
		return err
	}
	log.Printf("parsed %d routes", len(parsed))

	for _, r := range parsed {
		err := queries.UpsertRoute(ctx, db.UpsertRouteParams{
			RouteID:        r.RouteID,
			RouteShortName: r.RouteShortName,
			RouteLongName:  r.RouteLongName,
			RouteType:      r.RouteType,
		})
		if err != nil {
			return err
		}
	}

	log.Printf("upserted %d routes", len(parsed))
	return nil
}

func syncCalendar(ctx context.Context, client *mosru.Client, queries *db.Queries) error {
	log.Println("fetching calendar...")

	rows, err := client.FetchRows(datasetIDCalendar)
	if err != nil {
		return err
	}
	log.Printf("fetched %d raw rows", len(rows))

	parsed, err := mosru.ParseCalendar(rows)
	if err != nil {
		return err
	}
	log.Printf("parsed %d calendar entries", len(parsed))

	for _, c := range parsed {
		err := queries.UpsertCalendar(ctx, db.UpsertCalendarParams{
			ServiceID: c.ServiceID,
			Monday:    c.Monday,
			Tuesday:   c.Tuesday,
			Wednesday: c.Wednesday,
			Thursday:  c.Thursday,
			Friday:    c.Friday,
			Saturday:  c.Saturday,
			Sunday:    c.Sunday,
			StartDate: c.StartDate,
			EndDate:   c.EndDate,
		})
		if err != nil {
			return err
		}
	}

	log.Printf("upserted %d calendar entries", len(parsed))
	return nil
}

func syncTrips(ctx context.Context, client *mosru.Client, pool *pgxpool.Pool, queries *db.Queries) error {
	log.Println("fetching trips...")

	rows, err := client.FetchRows(datasetIDTrips)
	if err != nil {
		return err
	}
	log.Printf("fetched %d raw rows", len(rows))

	parsed, err := mosru.ParseTrips(rows)
	if err != nil {
		return err
	}
	log.Printf("parsed %d trips", len(parsed))

	knownRoutes, err := db.LoadExistingIDs(ctx, pool, "routes", "route_id")
	if err != nil {
		return err
	}
	knownServices, err := db.LoadExistingIDs(ctx, pool, "calendar", "service_id")
	if err != nil {
		return err
	}

	upserted := 0
	skipped := 0
	for _, t := range parsed {
		if !knownRoutes[t.RouteID] || !knownServices[t.ServiceID] {
			skipped++
			continue
		}

		err := queries.UpsertTrip(ctx, db.UpsertTripParams{
			TripID:       t.TripID,
			RouteID:      t.RouteID,
			ServiceID:    t.ServiceID,
			TripHeadsign: t.TripHeadsign,
			DirectionID:  t.DirectionID,
		})
		if err != nil {
			return err
		}
		upserted++
	}

	log.Printf("upserted %d trips, skipped %d (missing route_id/service_id in source data)", upserted, skipped)
	return nil
}

func syncStops(ctx context.Context, client *mosru.Client, queries *db.Queries) error {
	log.Println("fetching stops...")

	features, err := client.FetchFeatures(datasetIDStops)
	if err != nil {
		return err
	}
	log.Printf("fetched %d raw features", len(features))

	parsed, err := mosru.ParseStops(features)
	if err != nil {
		return err
	}
	log.Printf("parsed %d stops", len(parsed))

	for _, s := range parsed {
		err := queries.UpsertStop(ctx, db.UpsertStopParams{
			StopID:        s.StopID,
			StopName:      s.StopName,
			StopLat:       s.StopLat,
			StopLon:       s.StopLon,
			TransportType: s.TransportType,
			Street:        s.Street,
		})
		if err != nil {
			return err
		}
	}

	log.Printf("upserted %d stops", len(parsed))
	return nil
}

const stopTimesCheckpointKey = "stop_times"

func syncStopTimes(ctx context.Context, client *mosru.Client, pool *pgxpool.Pool) error {
	startSkip, err := db.GetCheckpoint(ctx, pool, stopTimesCheckpointKey)
	if err != nil {
		return err
	}
	if startSkip > 0 {
		log.Printf("resuming stop_times sync from skip=%d (checkpoint found)", startSkip)
	} else {
		log.Println("fetching stop_times (streaming, batch-inserting as we go)...")
	}

	const flushEvery = 20 // pages per staging flush (~10,000 rows at pageSize=500)

	var stagingBuf []db.StopTimeStagingRow
	pagesSinceFlush := 0
	totalFetched := 0
	totalInserted := int64(0)
	totalDropped := int64(0)
	lastSkip := startSkip

	flush := func(checkpointSkip int) error {
		if len(stagingBuf) == 0 {
			return nil
		}
		if _, err := db.BulkInsertStopTimesStaging(ctx, pool, stagingBuf); err != nil {
			return err
		}
		inserted, dropped, err := db.FlushStopTimesStaging(ctx, pool)
		if err != nil {
			return err
		}
		totalInserted += inserted
		totalDropped += dropped
		stagingBuf = stagingBuf[:0]
		pagesSinceFlush = 0

		// Save progress AFTER a successful flush, so a crash never
		// leaves the checkpoint ahead of what's actually in the DB.
		if err := db.SaveCheckpoint(ctx, pool, stopTimesCheckpointKey, checkpointSkip); err != nil {
			return err
		}

		log.Printf("  ...flushed batch: %d fetched so far, %d inserted total, %d dropped total, checkpoint=%d", totalFetched, totalInserted, totalDropped, checkpointSkip)
		return nil
	}

	err = client.FetchRowsStreaming(datasetIDStopTimes, startSkip, func(page []mosru.RawRow, currentSkip int) error {
		parsed, err := mosru.ParseStopTimes(page)
		if err != nil {
			return err
		}

		for _, p := range parsed {
			stagingBuf = append(stagingBuf, db.StopTimeStagingRow{
				GlobalID:       p.GlobalID,
				StopTimesIDRaw: p.StopTimesRaw,
				TripID:         p.TripID,
				StopID:         p.StopID,
				ArrivalTime:    p.ArrivalTime,
				DepartureTime:  p.DepartureTime,
				StopSequence:   p.StopSequence,
			})
		}
		totalFetched += len(parsed)
		pagesSinceFlush++
		lastSkip = currentSkip

		if pagesSinceFlush >= flushEvery {
			// checkpoint = next page to fetch after this one
			return flush(currentSkip + 500)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// flush any remainder smaller than a full batch
	if err := flush(lastSkip + 500); err != nil {
		return err
	}

	// Fully done — clear the checkpoint so a future run starts fresh
	// (e.g. next week's scheduled sync should re-scan from skip=0).
	if err := db.ClearCheckpoint(ctx, pool, stopTimesCheckpointKey); err != nil {
		return err
	}

	log.Printf("stop_times sync complete: %d fetched, %d inserted, %d dropped (missing trip_id/stop_id)", totalFetched, totalInserted, totalDropped)
	return nil
}
