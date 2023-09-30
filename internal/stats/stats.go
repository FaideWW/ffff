package stats

import (
	"context"
	"log"
	"os"
	"time"

	db "github.com/faideww/ffff/internal/db"
	"github.com/jackc/pgx/v5"
)

type JewelSnapshot struct {
	JewelType         string
	JewelClass        string
	AllocatedNode     string
	PriceBoxplot      Boxplot
	PriceBaseCurrency string
	NumListed         int
}

type Boxplot = [5]float64

func AggregateStats(from *time.Time, to *time.Time) error {
	l := log.New(os.Stdout, "[STATS]", log.Ldate|log.Ltime)
	ctx := context.Background()
	pqCfg := db.PQConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASS"),
		Dbname:   os.Getenv("DB_NAME"),
		Host:     os.Getenv("DB_HOST"),
		Sslmode:  "verify-full",
	}

	dbpool, err := db.DBConnect(&pqCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer dbpool.Close()

	// TODO: is there a nicer way to find leagues than a hardcoded env var?
	leagues := os.Getenv("LEAGUES")

	rows, err := dbpool.Query(ctx, "SELECT * FROM jewels WHERE league = any($1)", leagues)
	jewels, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.DBJewel])
	if err != nil {
		l.Printf("failed to collect rows\n")
		return err
	}

	return nil
}
