package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
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
	RecordedAt        time.Time `db:"recordedAt"`
}

func UpdateDb(ctx context.Context, db *pgxpool.Pool, stashes []StashSnapshot) error {
	l := log.New(os.Stdout, "[DB]", log.Ldate|log.Ltime)
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		l.Printf("failed to begin transaction\n")
		return err
	}
	defer tx.Rollback(ctx)

	stashesById := make(map[string]StashSnapshot)
	stashIds := make([]string, len(stashes))
	for i, t := range stashes {
		stashIds[i] = t.Id
		stashesById[t.Id] = t
	}

	rows, err := tx.Query(ctx, "SELECT (id,jewelType,jewelClass,allocatedNode,itemId,stashId,league,listPriceAmount,listPriceCurrency,recordedAt) FROM jewels WHERE stashId = any($1)", stashIds)
	if err != nil {
		l.Printf("failed to fetch entries\n")
		return err
	}

	txBatch := &pgx.Batch{}

	checkedItems := make(map[string]bool)
	jewels, err := pgx.CollectRows(rows, pgx.RowTo[DBJewel])
	if err != nil {
		l.Printf("failed to collect rows\n")
		return err
	}
	for _, j := range jewels {
		tab, ok := stashesById[j.StashId]
		if !ok {
			// how did this happen???
			return errors.New("somehow found a jewel with a non-indexed stash id (itemId=" + j.ItemId + ", stashId=" + j.StashId + ")")
		}

		// search for the jewel in the tab; if we find it, run the update listing routine. if not, delete it
		found := false
		for _, item := range tab.Items {
			if item.Id != j.ItemId {
				continue
			}
			found = true
			// check if needs to be updated
			if item.Price.Count != j.ListPriceAmount || item.Price.Currency != j.ListPriceCurrency {
				l.Printf("Price has changed for item %s (%f %s -> %f %s)\n", item, item.Price.Count, item.Price.Currency, j.ListPriceAmount, j.ListPriceCurrency)
				txBatch.Queue("UPDATE jewels SET listPriceAmount = $1, listPriceCurrency = $2, recordedAt = $3 WHERE id = $4", item.Price.Count, item.Price.Currency, tab.RecordedAt, j.Id)

			}

			checkedItems[item.Id] = true
			break
		}
		if !found {
			// delete the entry
			l.Printf("Item %s has been delisted, deleting entry\n", j.ItemId)
			txBatch.Queue("DELETE FROM jewels WHERE id = $1", j.Id)
		}
	}
	rows.Close()

	for _, tab := range stashes {
		// loop through stash and insert any remaining Items
		for _, item := range tab.Items {
			if _, ok := checkedItems[item.Id]; ok {
				continue
			}

			j := DBJewelConfig{
				item.Type, item.Class, item.Node, item.Id, tab.Id, tab.League, item.Price.Count, item.Price.Currency, tab.RecordedAt,
			}

			txBatch.Queue("INSERT INTO jewels(jewelType,jewelClass,allocatedNode,itemId,stashId,league,listPriceAmount,listPriceCurrency,recordedAt) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)", j.JewelType, j.JewelClass, j.AllocatedNode, j.ItemId, j.StashId, j.League, j.ListPriceAmount, j.ListPriceCurrency, j.RecordedAt)
			// nInsert, err := res.RowsAffected()

			l.Printf("Adding new item %s, at price %f %s\n", item, item.Price.Count, item.Price.Currency)
		}
	}

	if txBatch.Len() > 0 {
		res := tx.SendBatch(ctx, txBatch)
		// TODO: error handling?
		if err := res.Close(); err != nil {
			l.Printf("failed to close batch\n")
			return err
		}

	}

	if err := tx.Commit(ctx); err != nil {
		l.Printf("failed to commit transaction\n")
		return err
	}

	return nil
}
