package psapi

import (
	"context"
	"errors"
	"log"
	"os"

	db "github.com/faideww/ffff/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SELECT_JEWELS_QUERY = `
SELECT * 
  FROM jewels 
  WHERE stashId = any($1)
  `

const DELETE_JEWEL_QUERY = `
DELETE 
  FROM jewels 
  WHERE id = $1
  `

const UPDATE_JEWEL_PRICE_QUERY = `
UPDATE jewels 
  SET stashId = $1, listPriceAmount = $2, listPriceCurrency = $3, lastChangeId = $4, recordedAt = $5 
  WHERE id = $6
  `

const UPSERT_JEWEL_QUERY = `
INSERT INTO jewels(jewelType,jewelClass,allocatedNode,itemId,stashId,league,listPriceAmount,listPriceCurrency,lastChangeId,recordedAt) 
  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) 
  ON CONFLICT(itemId)
  DO
    UPDATE SET stashId = $5, listPriceAmount = $7, listPriceCurrency = $8, lastChangeId = $9, recordedAt = $10
`

func UpdateDb(ctx context.Context, dbHandle *pgxpool.Pool, stashes []StashSnapshot) error {
	l := log.New(os.Stdout, "[DB]", log.Ldate|log.Ltime)
	tx, err := dbHandle.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		l.Printf("failed to begin transaction\n")
		return err
	}
	defer tx.Rollback(ctx)

	changesetStashesById := make(map[string]StashSnapshot)
	changesetJewelsById := make(map[string]JewelEntry)
	changesetStashIds := make([]string, len(stashes))
	for i, t := range stashes {
		changesetStashIds[i] = t.Id
		changesetStashesById[t.Id] = t
		for _, j := range t.Items {
			changesetJewelsById[j.Id] = j
		}
	}

	rows, _ := tx.Query(ctx, SELECT_JEWELS_QUERY, changesetStashIds)
	// if err != nil {
	// l.Printf("failed once\n")
	// 	rows, err = tx.Query(ctx, SELECT_JEWELS_QUERY, changesetStashIds)
	// if err != nil {
	// l.Printf("failed again\n")
	// }
	// }
	defer rows.Close()

	checkedJewels := make(map[string]bool)
	dbJewels, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.DBJewel])
	if err != nil {
		l.Printf("failed to fetch entries\n")
		return err
	}

	batch := &pgx.Batch{}

	for _, dbJewel := range dbJewels {
		csJewel, jewelOk := changesetJewelsById[dbJewel.ItemId]
		csTab, tabOk := changesetStashesById[dbJewel.StashId]
		if tabOk && !jewelOk {
			// if the tab is found but not the jewel, we can assume it has been delisted and it's safe to delete the row
			l.Printf("Item %s has been delisted, deleting entry\n", dbJewel.ItemId)
			batch.Queue(DELETE_JEWEL_QUERY, dbJewel.Id)
		}

		// this should never happen, but just in case...
		if !tabOk && !jewelOk {
			return errors.New("somehow found a jewel with a non-indexed stash id (itemId=" + dbJewel.ItemId + ", stashId=" + dbJewel.StashId + ")")
		}

		// check if anything needs to be updated
		if tabOk && jewelOk && csJewel.Price.Count != dbJewel.ListPriceAmount || csJewel.Price.Currency != dbJewel.ListPriceCurrency || csTab.Id != dbJewel.StashId {
			l.Printf("Price has changed for item %s (%f %s -> %f %s)\n", csJewel, csJewel.Price.Count, csJewel.Price.Currency, dbJewel.ListPriceAmount, dbJewel.ListPriceCurrency)
			batch.Queue(UPDATE_JEWEL_PRICE_QUERY, csTab.Id, csJewel.Price.Count, csJewel.Price.Currency, csTab.ChangeId, csTab.RecordedAt, dbJewel.Id)
		}

		checkedJewels[dbJewel.ItemId] = true
	}
	rows.Close()

	for _, tab := range stashes {
		// loop through stash and insert any remaining Items
		for _, item := range tab.Items {
			if _, ok := checkedJewels[item.Id]; ok {
				continue
			}

			l.Printf("Adding new item %s, at price %f %s\n", item, item.Price.Count, item.Price.Currency)
			batch.Queue(UPSERT_JEWEL_QUERY, item.Type, item.Class, item.Node, item.Id, tab.Id, tab.League, item.Price.Count, item.Price.Currency, tab.ChangeId, tab.RecordedAt)
			// nInsert, err := res.RowsAffected()
		}
	}

	// TODO: error handling for batch requests
	results := tx.SendBatch(ctx, batch)
	results.Close()
	// if err != nil {
	//   l.Printf("failed to insert\n")
	//   return err
	// }

	err = tx.Commit(ctx)
	if err != nil {
		l.Printf("failed to commit transaction\n")
		return err
	}

	return nil
}
