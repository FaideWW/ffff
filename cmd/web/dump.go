package main

import (
	"cmp"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"github.com/faideww/ffff/internal/db"
	"github.com/faideww/ffff/internal/poeninja"
	"github.com/faideww/ffff/internal/stats"
	"github.com/jackc/pgx/v5"
)

type JewelDumpTemplateData struct {
	Test   string
	Jewels []db.DBJewel
}

type DumpTemplateData struct {
	Test   string
	Jewels []db.DBJewelSnapshot
}

func (s *FFFFServer) GetDump(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	league := r.PathValue("league")

	var setId int
	err := s.dbHandle.QueryRow(ctx, "SELECT id FROM snapshot_sets ORDER BY generatedAt DESC LIMIT 1").Scan(&setId)
	if err != nil {
		fmt.Println("failed to get latest snapshot set:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}

	var rows pgx.Rows
	if league == "" {
		rows, _ = s.dbHandle.Query(ctx, "SELECT * FROM snapshots WHERE setId = $1 ORDER BY windowPrice ASC", setId)
	} else {
		rows, _ = s.dbHandle.Query(ctx, "SELECT * FROM snapshots WHERE setId = $1 AND league = $2 ORDER BY windowPrice ASC", setId, league)
	}
	defer rows.Close()

	jewelData, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.DBJewelSnapshot])
	if err != nil {
		fmt.Println("failed to fetch from server:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}

	fmt.Printf("fetched %d snapshots\n", len(jewelData))

	// t, _ := template.New("test").Parse(`{{define "root"}}Hello {{ .World }}{{ end }}`)
	// t.ExecuteTemplate(w, "root", struct{ World string }{"world2"})

	lp := filepath.Join("templates/root.html")
	fp := filepath.Join("templates/dump.html")

	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		log.Fatal("failed to parse template:", err)
	}
	tmpl.ExecuteTemplate(w, "root", DumpTemplateData{
		Test:   "hello world",
		Jewels: jewelData,
	})
}

func (s *FFFFServer) GetCurrencyDump(w http.ResponseWriter, r *http.Request) {
	league := r.PathValue("league")
	res, err := poeninja.GetExchangeRates(&http.Client{Timeout: 30 * time.Second}, league)
	fmt.Printf("%+v\n", res)
	if err != nil {
		fmt.Println("failed to fetch from poeninja:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}
	var str string
	fmt.Sprintf(str, "%+v", res)
	io.WriteString(w, str)

}

func (s *FFFFServer) GetJewelDump(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	league := r.PathValue("league")
	jewelType := r.PathValue("jewelType")
	jewelNode := r.PathValue("nodeName")

	var exchangeRates map[string]map[string]float64
	err := s.dbHandle.QueryRow(ctx, "SELECT exchangeRates FROM snapshot_sets ORDER BY generatedAt DESC LIMIT 1").Scan(&exchangeRates)
	if err != nil {
		fmt.Println("failed to get latest snapshot set:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}

	var rows pgx.Rows
	if jewelNode != "" {
		rows, _ = s.dbHandle.Query(ctx, "SELECT * FROM jewels WHERE league = $1 AND jewelType = $2 AND allocatedNode = $3 ORDER BY recordedAt DESC", league, jewelType, jewelNode)
	} else {
		rows, _ = s.dbHandle.Query(ctx, "SELECT * FROM jewels WHERE league = $1 AND jewelType = $2 ORDER BY recordedAt DESC", league, jewelType)

	}
	defer rows.Close()

	jewelData, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.DBJewel])
	if err != nil {
		fmt.Println("failed to fetch from server:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}

	slices.SortFunc(jewelData, func(a, b db.DBJewel) int {
		aInC, _ := stats.GetPriceInChaos(&a, exchangeRates[league])
		bInC, _ := stats.GetPriceInChaos(&b, exchangeRates[league])

		return cmp.Compare(aInC, bInC)
	})

	lp := filepath.Join("templates/root.html")
	fp := filepath.Join("templates/jewelDump.html")

	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		log.Fatal("failed to parse template:", err)
	}
	tmpl.ExecuteTemplate(w, "root", JewelDumpTemplateData{
		Test:   "hello world",
		Jewels: jewelData,
	})
}
