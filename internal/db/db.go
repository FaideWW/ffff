package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PQConfig struct {
	User     string
	Password string
	Dbname   string
	Host     string
	Sslmode  string
}

type DBJewel struct {
	Id                int
	JewelType         string
	JewelClass        string
	AllocatedNode     string
	ItemId            string
	StashId           string
	League            string
	ListPriceAmount   float64
	ListPriceCurrency string
	LastChangeId      string
	RecordedAt        time.Time
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

func MakeConnectionString(c *PQConfig) string {
	return fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=%s", c.User, c.Password, c.Dbname, c.Host, c.Sslmode)
}

func DBConnect(c *PQConfig) (*pgxpool.Pool, error) {
	connStr := MakeConnectionString(c)
	dbpool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, err
	}

	return dbpool, nil
}
