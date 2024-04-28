package stats

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	db "github.com/faideww/ffff/internal/db"
	"github.com/faideww/ffff/internal/poeninja"
	"github.com/faideww/ffff/internal/utils"
	"github.com/jackc/pgx/v5"
)

type Boxplot = [5]float64

const DATE_CUTOFF = 48 * time.Hour

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

func GetPriceInChaos(j *db.DBJewel, rates map[string]float64) (int, bool) {
	if j.ListPriceCurrency == "chaos" {
		return int(j.ListPriceAmount), true
	}

	chaosRate, ok := rates[j.ListPriceCurrency]
	if !ok {
		fmt.Printf("unsupported currency %s (%f) for %s - %s\n", j.ListPriceCurrency, j.ListPriceAmount, j.JewelType, j.AllocatedNode)
		// unsupported currency, ignore
		return 0, false
	}
	chaosEquiv := int(j.ListPriceAmount * float64(chaosRate))
	if chaosEquiv == 0 {
		return 0, false
	}
	return chaosEquiv, true
}

func calculateStandardDeviation(values []float64) float64 {
	n := len(values)
	median := values[n/2]
	if n%2 == 0 {
		median = values[(n/2)-1] + values[n/2]/2
	}
	// calculate standard deviation
	sumDeviation := 0.0
	for _, v := range values {
		sumDeviation += (v - median) * (v - median)
	}
	return math.Sqrt(sumDeviation / float64(n))
}

func calculatePriceSpread(prices []int) ([5]float64, float64) {
	n := len(prices)
	floatPrices := make([]float64, n)
	for i, p := range prices {
		floatPrices[i] = float64(p)
	}
	pMin := floatPrices[0]
	pMax := floatPrices[n-1]
	pMed := floatPrices[n/2]
	pQ1 := floatPrices[n/4]
	pQ3 := floatPrices[(3*n)/4]
	stddev := calculateStandardDeviation(floatPrices)

	return [5]float64{pMin, pQ1, pMed, pQ3, pMax}, stddev
}

func calculateWindowPriceStddev(prices []int, boxplot [5]float64, stddev float64) float64 {
	meanMinusOne := boxplot[2] - stddev
	meanPlusOne := boxplot[2] + stddev

	filteredPrices := []int{}

	for _, p := range prices {
		if float64(p) >= meanMinusOne && float64(p) <= meanPlusOne && p >= 1 {
			filteredPrices = append(filteredPrices, p)
		}
	}
	// recalculate stddev and filter again
	nextBox, nextStddev := calculatePriceSpread(filteredPrices)
	meanMinusOne = nextBox[2] - nextStddev
	meanPlusOne = nextBox[2] + nextStddev

	doubleFilteredPrices := []int{}

	for _, p := range filteredPrices {
		if float64(p) >= meanMinusOne && float64(p) <= meanPlusOne {
			doubleFilteredPrices = append(doubleFilteredPrices, p)
		}
	}

	return float64(doubleFilteredPrices[0])
}

// https://www.itl.nist.gov/div898/handbook/eda/section3/eda35h.htm - modified Z-score
// https://eurekastatistics.com/using-the-median-absolute-deviation-to-find-outliers/
func calculateWindowPriceMAD(prices []int, w *bufio.Writer) float64 {
	median := prices[len(prices)/2]
	var deviationsLeft []int
	var deviationsRight []int
	for _, p := range prices {
		if p <= median {
			diff := (median - p)
			deviationsLeft = append(deviationsLeft, diff)
		}
		if p >= median {
			diff := (p - median)
			deviationsRight = append(deviationsRight, diff)
		}
	}

	slices.Sort(deviationsLeft)
	slices.Sort(deviationsRight)
	medianDeviationLeft := deviationsLeft[len(deviationsLeft)/2]
	medianDeviationRight := deviationsRight[len(deviationsRight)/2]
	// if medianDeviationLeft == 0 {
	//   return float64(median)
	// }

	mThreshold := 2.0

	distances := make([]float64, len(prices))
	var inliers []int
	for i, x := range prices {
		mad := 0.0
		if x != median {
			if x < median {
				mad = float64(medianDeviationLeft)
			} else if x > median {
				mad = float64(medianDeviationRight)
			}
			distance := (float64(x) - float64(median)) / mad
			if distance < 0 {
				distance *= -1
			}
			distances[i] = distance
		} else {
			distances[i] = 0.0
		}
		if distances[i] < mThreshold {
			inliers = append(inliers, x)
		}
	}

	fmt.Fprintf(w, " - prices: %+v\n", prices)
	fmt.Fprintf(w, " - deviations left: %+v\n", deviationsLeft)
	fmt.Fprintf(w, " - deviations right: %+v\n", deviationsRight)
	fmt.Fprintf(w, " - median: %d - mad (left): %d - mad (right): %d\n", median, medianDeviationLeft, medianDeviationRight)
	fmt.Fprintf(w, " - zscores: %+v\n", distances)
	fmt.Fprintf(w, " - inliers: %+v\n", inliers)
	if len(inliers) > 1 {
		return float64(inliers[1])
	}
	return float64(inliers[0])
}

func calculateWindowPriceClustered(prices []int, w *bufio.Writer) (float64, float64, error) {
	floatPrices := make([]float64, len(prices))
	for i, p := range prices {
		floatPrices[i] = float64(p)
	}
	clusters := HCluster(floatPrices)

	var inliers [][]float64
	var minClusterSize = 3

	for len(inliers) == 0 && minClusterSize > 0 {
		for _, c := range clusters {
			if len(c) >= minClusterSize {
				inliers = append(inliers, c)
			}
		}
		minClusterSize--
	}

	if len(inliers) == 0 {
		fmt.Printf("clusters: %+v\n", clusters)
		fmt.Printf("inliers: %+v\n", inliers)
		return 0, 0, errors.New("found 0 inliers")
	}

	for _, c := range inliers {
		slices.Sort(c)
	}

	slices.SortFunc(inliers, func(a, b []float64) int {
		res := a[0] - b[0]
		if res < 0 {
			return -1
		} else if res == 0 {
			return 0
		} else {
			return 1
		}
	})

	fmt.Fprintf(w, " - prices: %+v\n", prices)
	fmt.Fprintf(w, " - clusters: %+v\n", clusters)
	fmt.Fprintf(w, " - inliers: %+v\n", inliers)

	// Use the median value of the cluster
	// Estimate confidence as a function of the cluster size
	targetCluster := inliers[0]
	const highConfClusterSize = 10.0
	confidence := math.Min(float64(len(targetCluster))/highConfClusterSize, 1.0)

	return targetCluster[len(targetCluster)/2], confidence, nil
}

func AggregateStats() error {
	start := time.Now()
	l := log.New(os.Stdout, "[STATS]", log.Ldate|log.Ltime)
	ctx := context.Background()
	dbHandle, err := db.DBConnect(os.Getenv("PG_DB_CONNSTR"))
	if err != nil {
		log.Fatal(err)
	}
	defer dbHandle.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	// TODO: is there a nicer way to find leagues than a hardcoded env var?
	leagues := strings.Split(os.Getenv("LEAGUES"), ",")

	exchangeRates := make(map[string]map[string]float64, len(leagues))
	for _, league := range leagues {
		rates, ratesErr := poeninja.GetExchangeRates(client, league)
		if ratesErr != nil {
			l.Printf("Failed to retrieve exchange rates for league %s\n", league)
			return err
		}
		exchangeRates[league] = rates
	}

	jewelFetchStart := time.Now()
	dateCutoff := time.Now().Add(-DATE_CUTOFF)
	rows, _ := dbHandle.Query(ctx, "SELECT * FROM jewels WHERE league = any($1) AND recordedat > ($2)", leagues, dateCutoff)
	defer rows.Close()
	jewelFetchElapsed := time.Since(jewelFetchStart)
	l.Printf("db fetch took %.3fs\n", jewelFetchElapsed.Seconds())

	aggStart := time.Now()

	jewels, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.DBJewel])
	if err != nil {
		l.Printf("failed to collect rows\n")
		return err
	}

	l.Printf("row marshalling took %.3fs\n", time.Since(aggStart).Seconds())
	jewelPrices := make(map[string][]int)
	seenCurrencies := make(map[string]map[string]bool)
	for _, j := range jewels {

		jKey := hashJewelKey(&j)
		_, keyOk := jewelPrices[jKey]
		price, priceOk := GetPriceInChaos(&j, exchangeRates[j.League])
		if _, leagueOk := seenCurrencies[j.League]; !leagueOk {
			seenCurrencies[j.League] = make(map[string]bool)
		}
		if priceOk {
			if !keyOk {
				jewelPrices[jKey] = []int{}
			}
			jewelPrices[jKey] = utils.InsertSorted(jewelPrices[jKey], price)
			seenCurrencies[j.League][j.ListPriceCurrency] = true
		}
	}
	parseTime := time.Since(start)

	l.Printf("Parsing %d jewels took %s\n", len(jewels), parseTime)

	tx, err := dbHandle.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	fmt.Printf("seenCurrencies:%+v\n", seenCurrencies)
	// seenExchangeRates := make(map[string]map[string]float64)
	setIdsByLeague := make(map[string]int)
	for league, rates := range seenCurrencies {
		leagueRates := make(map[string]float64)
		for currency, cOk := range rates {
			if cOk {
				leagueRates[currency] = exchangeRates[league][currency]
			}
		}
		var setId int
		exchangeRatesJson, marshalErr := json.Marshal(leagueRates)
		if marshalErr != nil {
			l.Printf("failed to marshal exchange rates\n")
			return err
		}
		err = tx.QueryRow(ctx, "INSERT INTO snapshot_sets(league, exchangeRates, generatedAt) VALUES (@league, @exchangeRates, @generatedAt) RETURNING id",
			pgx.NamedArgs{
				"league":        league,
				"exchangeRates": exchangeRatesJson,
				"generatedAt":   start,
			}).Scan(&setId)
		if err != nil {
			l.Printf("failed to create new snapshot set\n")
			return err
		}
		setIdsByLeague[league] = setId
	}

	debugFile, err := os.Create("stats.txt")
	if err != nil {
		l.Printf("failed to create file\n")
		return err
	}
	defer debugFile.Close()
	w := bufio.NewWriter(debugFile)
	defer w.Flush()

	batch := &pgx.Batch{}
	for k, p := range jewelPrices {
		jData := unhashJewelKey(k)
		fmt.Fprintf(w, "%+v\n", jData)
		boxplot, stddev := calculatePriceSpread(p)
		setId := setIdsByLeague[jData.League]
		// windowPrice := calculateWindowPriceStddev(p, boxplot, stddev)
		// windowPrice := calculateWindowPriceMAD(p, w)
		windowPrice, confidence, priceErr := calculateWindowPriceClustered(p, w)
		if priceErr != nil {
			return priceErr
		}

		s := db.DBJewelSnapshot{
			SetId:              setId,
			JewelType:          jData.JewelType,
			JewelClass:         jData.JewelClass,
			AllocatedNode:      jData.AllocatedNode,
			MinPrice:           boxplot[0],
			FirstQuartilePrice: boxplot[1],
			MedianPrice:        boxplot[2],
			ThirdQuartilePrice: boxplot[3],
			MaxPrice:           boxplot[4],
			WindowPrice:        windowPrice,
			Confidence:         confidence,
			Stddev:             stddev,
			NumListed:          len(p),
			GeneratedAt:        start,
		}

		batch.Queue("INSERT INTO snapshots(setId,jewelType,jewelClass,allocatedNode,minPrice,firstQuartilePrice,medianPrice,thirdQuartilePrice,maxPrice,windowPrice,stddev,numListed,generatedAt) VALUES (@setId,@jewelType,@jewelClass,@allocatedNode,@minPrice,@q1Price,@medianPrice,@q3Price,@maxPrice,@windowPrice,@stddev,@numListed,@generatedAt)", pgx.NamedArgs{
			"setId":         s.SetId,
			"jewelType":     s.JewelType,
			"jewelClass":    s.JewelClass,
			"allocatedNode": s.AllocatedNode,
			"minPrice":      s.MinPrice,
			"q1Price":       s.FirstQuartilePrice,
			"medianPrice":   s.MedianPrice,
			"q3Price":       s.ThirdQuartilePrice,
			"maxPrice":      s.MaxPrice,
			"windowPrice":   s.WindowPrice,
			"stddev":        s.Stddev,
			"numListed":     s.NumListed,
			"generatedAt":   s.GeneratedAt,
		})
		// l.Printf("%s: %v (%f)\n", k, boxplot, stddev)
	}

	results := tx.SendBatch(ctx, batch)
	for range jewelPrices {
		_, err = results.Exec()
		if err != nil {
			fmt.Printf("failed to insert snapshot\n")
			return err
		}
	}
	results.Close()

	err = tx.Commit(ctx)
	if err != nil {
		l.Printf("failed to commit tx\n")
		return err
	}

	l.Printf("Aggregated %d listings into %d entries in %.2fs\n", len(jewels), len(jewelPrices), time.Since(start).Seconds())

	return nil
}
