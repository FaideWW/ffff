package stats

import (
	"context"
	"log"
	"os"
	"time"

	db "github.com/faideww/ffff/internal/db"
	"github.com/jmoiron/sqlx"
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
	dbCfg := db.SQLiteConfig{
		DbUrl:       os.Getenv("DB_URL"),
		DbAuthToken: os.Getenv("DB_AUTHTOKEN"),
	}

	dbHandle, err := db.DBConnect(&dbCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer dbHandle.Close()

	// TODO: is there a nicer way to find leagues than a hardcoded env var?
	leagues := os.Getenv("LEAGUES")

	query, args, err := sqlx.In("SELECT * FROM jewels WHERE league IN (?)", leagues)
	if err != nil {
		l.Printf("failed to expand slice in fetch entries query\n")
		return err
	}
	jewels := []db.DBJewel{}
	err = dbHandle.GetContext(ctx, jewels, query, args...)
	if err != nil {
		l.Printf("failed to collect rows\n")
		return err
	}

	return nil
}
