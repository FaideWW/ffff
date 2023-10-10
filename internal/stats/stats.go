package stats

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	db "github.com/faideww/ffff/internal/db"
	"github.com/jmoiron/sqlx"
	"golang.org/x/exp/slices"
)

// TODO: track this on the river and fetch the live value from the database
// when we run the stat calculations
const CHAOS_PER_DIVINE = 230

type Boxplot = [5]float64

func hashJewelKey(j *db.DBJewel) string {
	return fmt.Sprintf("%s_%s_%s_%s", j.League, j.JewelType, j.JewelClass, j.AllocatedNode)
}

func unhashJewelKey(key string) db.DBJewel {
	props := strings.Split(key, "_")
	return db.DBJewel{
		League:        props[0],
		JewelType:     props[1],
		JewelClass:    props[2],
		AllocatedNode: props[3],
	}
}

func getPriceInChaos(j *db.DBJewel, exchangeRate int) (int, bool) {
	if j.ListPriceCurrency == "chaos" {
		return int(j.ListPriceAmount), true
	} else if j.ListPriceCurrency == "divine" {
		return int(j.ListPriceAmount * float64(exchangeRate)), true
	}

	// unsupported currency, ignore
	return 0, false
}

func insertSorted(arr []int, v int) []int {
	pos, _ := slices.BinarySearch(arr, v)
	arr = slices.Insert(arr, pos, v)
	return arr
}

func calculatePriceSpread(prices []int) ([5]float64, float64) {
	n := len(prices)
	pMin := float64(prices[0])
	pMax := float64(prices[n-1])
	pMed := float64(prices[n/2])
	if n%2 == 0 {
		pMed = float64(prices[(n/2)-1]+prices[n/2]) / 2
	}
	pQ1 := float64(prices[n/4])
	pQ3 := float64(prices[(3*n)/4])

	// calculate standard deviation
	sumDeviation := 0.0
	for _, v := range prices {
		sumDeviation += math.Pow(float64(v)-float64(pMed), 2)
	}
	stddev := math.Sqrt(sumDeviation / float64(n))

	return [5]float64{pMin, pQ1, pMed, pQ3, pMax}, stddev

}

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
	leagues := strings.Split(os.Getenv("LEAGUES"), ",")

	query, args, err := sqlx.In("SELECT * FROM jewels WHERE league IN (?)", leagues)
	if err != nil {
		l.Printf("failed to expand slice in fetch entries query\n")
		return err
	}

	exchangeRate := CHAOS_PER_DIVINE

	jewelPrices := make(map[string][]int)

	rows, err := dbHandle.QueryxContext(ctx, query, args...)
	if err != nil {
		l.Printf("failed to collect rows\n")
		return err
	}
	defer rows.Close()

	jCount := 0
	start := time.Now()
	for rows.Next() {
		j := db.DBJewel{}
		err = rows.StructScan(&j)
		if err != nil {
			l.Printf("failed to scan struct\n")
			return err
		}
		jCount++

		jKey := hashJewelKey(&j)
		_, ok := jewelPrices[jKey]
		if !ok {
			jewelPrices[jKey] = []int{}
		}
		price, ok := getPriceInChaos(&j, exchangeRate)
		if ok {
			jewelPrices[jKey] = insertSorted(jewelPrices[jKey], price)
		}
	}
	parseTime := time.Since(start)

	l.Printf("Parsing %d jewels took %s\n", jCount, parseTime)

	tx, err := dbHandle.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// snapshots := make([]JewelSnapshot, len(jewelPrices))

	for k, p := range jewelPrices {
		jData := unhashJewelKey(k)
		boxplot, stddev := calculatePriceSpread(p)
		s := db.DBJewelSnapshot{
			League:             jData.League,
			JewelType:          jData.JewelType,
			JewelClass:         jData.JewelClass,
			AllocatedNode:      jData.AllocatedNode,
			MinPrice:           boxplot[0],
			FirstQuartilePrice: boxplot[1],
			MedianPrice:        boxplot[2],
			ThirdQuartilePrice: boxplot[3],
			MaxPrice:           boxplot[4],
			Stddev:             stddev,
			NumListed:          len(p),
			ExchangeRate:       exchangeRate,
			GeneratedAt:        time.Now(),
		}

		_, err = tx.NamedExecContext(ctx, "INSERT INTO snapshots(league,jewelType,jewelClass,allocatedNode,minPrice,firstQuartilePrice,medianPrice,thirdQuartilePrice,maxPrice,stddev,numListed,exchangeRate,generatedAt) VALUES (:league,:jewelType,:jewelClass,:allocatedNode,:minPrice,:firstQuartilePrice,:medianPrice,:thirdQuartilePrice,:maxPrice,:stddev,:numListed,:exchangeRate,:generatedAt)", s)
		if err != nil {
			fmt.Printf("failed to insert snapshot\n")
			return err
		}
		// snapshots = append(snapshots, s)

		// l.Printf("%s: %v (%f)\n", k, boxplot, stddev)
	}

	err = tx.Commit()
	if err != nil {
		l.Printf("failed to commit tx\n")
		return err
	}

	return nil
}

// type ByNode = map[string][]Price
// type ByClass = map[string]ByNode
// type ByType = map[string]ByClass
// type ByLeague = map[string]ByType
// func ensurePath(j *db.DBJewel, m map[string]ByLeague) {
//   league, ok := m[j.League]

//   if !ok {
//     league = make(ByLeague)
//     m[j.League] = league
//   }

//   jewelType, ok := m[j.League][j.JewelType]

//   if !ok {
//     jewelType := make(ByType)
//     m[j.League][j.JewelType] = jewelType
//   }

//   jewelClass, ok := m[j.League][j.JewelType][j.JewelClass]

//   if !ok {
//     jewelClass := make(ByClass)
//     m[j.League][j.JewelType][j.JewelClass] = jewelClass
//   }

//   allocatedNode, ok := m[j.League][j.JewelType][j.JewelClass][j.AllocatedNode]

//   if !ok {
//     allocatedNode := make([]Price)
//     m[j.League][j.JewelType][j.JewelClass][j.AllocatedNode] = jewelClass
//   }
// }
