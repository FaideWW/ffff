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

func MakeConnectionString(c *SQLiteConfig) string {
	return fmt.Sprintf("%s?authToken=%s", c.DbUrl, c.DbAuthToken)
}

func DBConnect(c *SQLiteConfig) (*sqlx.DB, error) {
	connStr := MakeConnectionString(c)

	db, err := sqlx.Connect("libsql", connStr)
	return db, err
}
