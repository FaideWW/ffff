package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/libsql/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

type SQLiteConfig struct {
	DbUrl       string
	DbAuthToken string
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

type DBJewelConfig struct {
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
	Id           int           `db:"id"`
	ChangeId     string        `db:"changeId"`
	NextChangeId string        `db:"nextChangeId"`
	StashCount   int           `db:"stashCount"`
	ProcessedAt  time.Time     `db:"processedAt"`
	TimeTaken    time.Duration `db:"timeTaken"`
}

type DBJewelSnapshot struct {
	Id                 int       `db:"id"`
	League             string    `db:"league"`
	JewelType          string    `db:"jewelType"`
	JewelClass         string    `db:"jewelClass"`
	AllocatedNode      string    `db:"allocatedNode"`
	MinPrice           float64   `db:"minPrice"`
	FirstQuartilePrice float64   `db:"firstQuartilePrice"`
	MedianPrice        float64   `db:"medianPrice"`
	ThirdQuartilePrice float64   `db:"thirdQuartilePrice"`
	MaxPrice           float64   `db:"maxPrice"`
	Stddev             float64   `db:"stddev"`
	NumListed          int       `db:"numListed"`
	ExchangeRate       int       `db:"exchangeRate"`
	GeneratedAt        time.Time `db:"generatedAt"`
}

func MakeConnectionString(c *SQLiteConfig) string {
	return fmt.Sprintf("%s?authToken=%s", c.DbUrl, c.DbAuthToken)
}

func DBConnect(c *SQLiteConfig) (*sqlx.DB, error) {
	connStr := MakeConnectionString(c)

	db, err := sqlx.Connect("libsql", connStr)
	return db, err
}
