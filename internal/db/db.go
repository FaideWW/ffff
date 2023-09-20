package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PQConfig struct {
	User     string
	Password string
	Dbname   string
	Host     string
	Sslmode  string
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
