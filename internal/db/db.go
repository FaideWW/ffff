package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// type SQLiteConfig struct {
// 	DbUrl       string
// 	DbAuthToken string
// }

type PGXScanner interface {
	Scan(dest ...interface{}) (err error)
}

type DBJewel struct {
	Id                int       `db:"id"`
	JewelType         string    `db:"jewelType"`
	JewelClass        string    `db:"jewelClass"`
	AllocatedNode     string    `db:"allocatedNode"`
	ItemId            string    `db:"itemId"`
	StashId           string    `db:"stashId"`
	League            string    `db:"league"`
	ListPriceAmount   float64   `db:"listPriceAmount"`
	ListPriceCurrency string    `db:"listPriceCurrency"`
	LastChangeId      string    `db:"lastChangeId"`
	RecordedAt        time.Time `db:"recordedAt"`
}

type DBChangeset struct {
	Id           int       `db:"id"`
	ChangeId     string    `db:"changeId"`
	NextChangeId string    `db:"nextChangeId"`
	StashCount   int       `db:"stashCount"`
	ProcessedAt  time.Time `db:"processedAt"`
	TimeTakenMs  int64     `db:"timeTaken"`
}

type DBSnapshotSet struct {
	Id            int                `db:"id"`
	ExchangeRates map[string]float64 `db:"exchangeRates"`
	GeneratedAt   time.Time          `db:"generatedAt"`
}

type DBJewelSnapshot struct {
	Id                 int       `db:"id"`
	SetId              int       `db:"setId"`
	JewelType          string    `db:"jewelType"`
	JewelClass         string    `db:"jewelClass"`
	League             string    `db:"league"`
	AllocatedNode      string    `db:"allocatedNode"`
	MinPrice           float64   `db:"minPrice"`
	FirstQuartilePrice float64   `db:"firstQuartilePrice"`
	MedianPrice        float64   `db:"medianPrice"`
	ThirdQuartilePrice float64   `db:"thirdQuartilePrice"`
	MaxPrice           float64   `db:"maxPrice"`
	WindowPrice        float64   `db:"windowPrice"`
	Stddev             float64   `db:"stddev"`
	NumListed          int       `db:"numListed"`
	GeneratedAt        time.Time `db:"generatedAt"`
}

func DBConnect(connStr string) (*pgxpool.Pool, error) {
	db, err := pgxpool.New(context.Background(), connStr)
	return db, err
}
