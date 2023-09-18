package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
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

func DBConnect(c *PQConfig) (*sql.DB, error) {
	connStr := MakeConnectionString(c)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return db, nil
}

type DBJewel struct {
	Id               int
	JewelType        string
	JewelClass       string
	AllocatedNode    string
	StashX           int
	StashY           int
	ItemId           string
	StashId          string
	ListPriceChaos   float64
	ListPriceDivines float64
	RecordedAt       time.Time
}

/*
For each stash snapshot:
  - fetch the db entries for that stash id
  - for each db entry:
  - find the matching item in the snapshot (by id, then x/y/type/node)
  - if no matching item is found, it's no longer listed. delete the db entry
  - if a matching item is found, check if any values need to be updated (price, location, recordedAt) and update them
  - mark the item as inserted
  - for each item in the snapshot:
  - if the item hasn't already been marked, insert a new entry
*/
func UpdateDb(ctx context.Context, db *sql.DB, tabs []StashSnapshot) error {
	l := log.New(os.Stdout, "[DB]", log.Ldate|log.Ltime)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	fetchEntriesStmt, err := tx.Prepare("SELECT id,jewelType,jewelClass,allocatedNode,stashX,stashY,itemId,stashId,listPriceChaos,listPriceDivines,recordedAt FROM jewels WHERE stashId = $1")
	if err != nil {
		return err
	}

	updatePriceStmt, err := tx.Prepare("UPDATE jewels SET stashX = $1, stashY = $2, listPriceDivines = $3, listPriceChaos = $4, recordedAt = $5 WHERE id = $6")
	if err != nil {
		return err
	}

	deleteEntryStmt, err := tx.Prepare("DELETE FROM jewels WHERE id = $1")
	if err != nil {
		return err
	}

	insertEntryStmt, err := tx.Prepare("INSERT INTO jewels(jewelType,jewelClass,allocatedNode,stashX,stashY,itemId,stashId,listPriceChaos,listPriceDivines,recordedAt) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)")

	for i, tab := range tabs {
		l.Printf("Syncing tab %s with db (%d of %d)\n", tab.Id, i, len(tabs))
		var entries []DBJewel
		rows, err := fetchEntriesStmt.QueryContext(ctx, tab.Id)

		for rows.Next() {
			var j DBJewel
			if err := rows.Scan(&j.Id, &j.JewelType, &j.JewelClass, &j.AllocatedNode, &j.StashX, &j.StashY, &j.ItemId, &j.StashId, &j.ListPriceChaos, &j.ListPriceDivines, &j.RecordedAt); err != nil {
				return err
			}

			entries = append(entries, j)
		}

		if err = rows.Err(); err != nil {
			return err
		}

		l.Printf("Found %d tracked entries for tab\n", len(entries))

		checkedItems := make(map[string]bool)

		for _, entry := range entries {
			found := false
			for _, item := range tab.Items {
				if item.Id == entry.ItemId {
					found = true
					// check if needs to be updated
					var err error
					if item.Price.Currency == "divine" && item.Price.Count != entry.ListPriceDivines {
						l.Printf("Price has changed for item %s (%f %s -> %f %s)\n", item, item.Price.Count, item.Price.Currency, entry.ListPriceDivines, "divine")
						_, err = updatePriceStmt.ExecContext(ctx, item.StashX, item.StashY, item.Price.Count, nil, tab.RecordedAt, entry.Id)
					} else if item.Price.Currency == "chaos" && item.Price.Count != entry.ListPriceChaos {
						l.Printf("Price has changed for item %s (%f %s -> %f %s)\n", item, item.Price.Count, item.Price.Currency, entry.ListPriceChaos, "chaos")
						_, err = updatePriceStmt.ExecContext(ctx, item.StashX, item.StashY, nil, item.Price.Count, tab.RecordedAt, entry.Id)
					}

					if err != nil {
						return err
					}

					checkedItems[item.Id] = true

					break
				}
			}

			if !found {
				// delete the entry
				l.Printf("Item %s has been delisted, deleting entry\n", entry.ItemId)
				_, err := deleteEntryStmt.ExecContext(ctx, entry.Id)
				if err != nil {
					return err
				}
			}
		}

		// loop through stash and insert any remaining Items
		for _, item := range tab.Items {
			if checkedItems[item.Id] {
				continue
			}

			var err error
			if item.Price.Currency == "divine" {
				_, err = insertEntryStmt.ExecContext(ctx, item.Type, item.Class, item.Node, item.StashX, item.StashY, item.Id, tab.Id, nil, item.Price.Count, tab.RecordedAt)
			} else if item.Price.Currency == "chaos" {
				_, err = insertEntryStmt.ExecContext(ctx, item.Type, item.Class, item.Node, item.StashX, item.StashY, item.Id, tab.Id, item.Price.Count, nil, tab.RecordedAt)
			}
			l.Printf("Adding new item %s, at price %f %s\n", item, item.Price.Count, item.Price.Currency)

			if err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
