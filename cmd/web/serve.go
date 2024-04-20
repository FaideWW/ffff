package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/faideww/ffff/internal/db"
	"github.com/faideww/ffff/internal/stats"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CliFlags struct{}

type FFFFServer struct {
	dbHandle *pgxpool.Pool
}

func NewServer() (*FFFFServer, error) {
	s := FFFFServer{}
	db, err := db.DBConnect(os.Getenv("PG_DB_CONNSTR"))
	if err != nil {
		return nil, err
	}

	s.dbHandle = db

	return &s, nil
}

type JewelPrice struct {
	ChaosEquivalent float64
	Amount          float64
	Currency        string
}

type JewelData struct {
	AllocatedNode string
	JewelClass    string
	FleshPrice    JewelPrice
	FlamePrice    JewelPrice
}

func (s *FFFFServer) FetchJewelData(ctx context.Context, league string) ([]JewelData, error) {
	var setId int
	var exchangeRates map[string]float64
	err := s.dbHandle.QueryRow(ctx, "SELECT id, exchangeRates FROM snapshot_sets ORDER BY generatedAt DESC LIMIT 1").Scan(&setId, &exchangeRates)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if league == "" {
		rows, _ = s.dbHandle.Query(ctx, "SELECT * FROM snapshots WHERE setId = $1 ORDER BY windowPrice ASC", setId)
	} else {
		rows, _ = s.dbHandle.Query(ctx, "SELECT * FROM snapshots WHERE setId = $1 AND league = $2 ORDER BY windowPrice ASC", setId, league)
	}
	defer rows.Close()

	dbJewelData, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.DBJewelSnapshot])
	if err != nil {
		return nil, err
	}

	var jewelMap map[string]JewelData
	for _, dbJewel := range dbJewelData {
		j, ok := jewelMap[dbJewel.AllocatedNode]
		if !ok {
			j = JewelData{
				AllocatedNode: dbJewel.AllocatedNode,
				JewelClass:    dbJewel.JewelClass,
				FleshPrice:    JewelPrice{0, 0, ""},
				FlamePrice:    JewelPrice{0, 0, ""},
			}
			jewelMap[dbJewel.AllocatedNode] = j
		}

		price := JewelPrice{dbJewel.WindowPrice, dbJewel.WindowPrice, "chaos"}
		if price.Amount > exchangeRates["mirror"] {
			price.Amount /= exchangeRates["mirror"]
			price.Currency = "mirror"
		} else if price.Amount > exchangeRates["divine"] {
			price.Amount /= exchangeRates["divine"]
			price.Currency = "divine"
		}

		if dbJewel.JewelType == "Forbidden Flesh" {
			j.FleshPrice = price
		} else {
			j.FlamePrice = price
		}
	}

	fmt.Printf("jewelMap: %+v\n", jewelMap)

	var result []JewelData
	for _, j := range jewelMap {
		result = stats.InsertSortedFunc(result, j, func(a, b JewelData) int {
			return int(
				(a.FlamePrice.ChaosEquivalent + a.FleshPrice.ChaosEquivalent) -
					(b.FlamePrice.ChaosEquivalent + b.FleshPrice.ChaosEquivalent))
		})
	}

	return result, nil
}

func FormatCurrency(p JewelPrice) string {
	return fmt.Sprintf("%.0f %s", p.Amount, p.Currency)
}

type HomeTemplateData struct {
	Jewels []JewelData
}

func (s *FFFFServer) GetHome(w http.ResponseWriter, r *http.Request) {
	league := r.PathValue("league")
	if league == "" {
		league = os.Getenv("DEFAULT_LEAGUE")
	}
	if league == "" {
		league = "Standard"
	}
	lp := filepath.Join("templates/root.html")
	fp := filepath.Join("templates/home.html")

	jewelData, err := s.FetchJewelData(r.Context(), r.PathValue(""))
	if err != nil {
		fmt.Println("failed to get jewel data:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}
	fmt.Printf("jewelData: %+v\n", jewelData)

	tmpl, err := template.New("root").Funcs(template.FuncMap{
		"currency": FormatCurrency,
	}).ParseFiles(lp, fp)
	if err != nil {
		log.Fatal("failed to parse template:", err)
	}
	tmpl.ExecuteTemplate(w, "root", HomeTemplateData{Jewels: jewelData})
}

func StartWebServer(port string) {
	s, err := NewServer()
	if err != nil {
		fmt.Printf("failed to instantiate server")
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("./static"))

	http.HandleFunc("/", s.GetHome)
	if os.Getenv("GO_ENV") == "" {
		http.HandleFunc("/dump", s.GetDump)
		http.HandleFunc("/dump/{$}", s.GetDump)
		http.HandleFunc("/dump/currency/{league}", s.GetCurrencyDump)
		http.HandleFunc("/dump/{league}", s.GetDump)
		http.HandleFunc("GET /dump/jewel/{league}/{jewelType}/{nodeName}", s.GetJewelDump)
	}
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	fmt.Printf("Server listening on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}

// func getRoot(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()

// 	fmt.Println("server: root handler started")
// 	defer fmt.Println("server root handler ended")

// 	select {
// 	case <-time.After(10 * time.Second):
// 		fmt.Fprintf(w, "hello\n")
// 	case <-ctx.Done():
// 		err := ctx.Err()
// 		fmt.Println("server:", err)
// 		internalError := http.StatusInternalServerError
// 		http.Error(w, err.Error(), internalError)
// 	}
// }
