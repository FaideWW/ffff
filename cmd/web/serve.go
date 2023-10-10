package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/faideww/ffff/internal/db"
	"github.com/jmoiron/sqlx"
)

type CliFlags struct{}

type FFFFServer struct {
	dbHandle *sqlx.DB
}

type DumpTemplateData struct {
	Test   string
	Jewels []db.DBJewelSnapshot
}

func NewServer() (*FFFFServer, error) {
	s := FFFFServer{}
	db, err := db.DBConnect(&db.SQLiteConfig{
		DbUrl:       os.Getenv("DB_URL"),
		DbAuthToken: os.Getenv("DB_AUTHTOKEN"),
	})
	if err != nil {
		return nil, err
	}

	s.dbHandle = db

	return &s, nil
}

func (s *FFFFServer) GetRoot(w http.ResponseWriter, r *http.Request) {
	lp := filepath.Join("templates/root.html")
	fp := filepath.Join("templates/home.html")

	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		log.Fatal("failed to parse template:", err)
	}
	tmpl.ExecuteTemplate(w, "root", nil)
}

func (s *FFFFServer) GetDump(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.dbHandle.QueryxContext(ctx, "SELECT * FROM snapshots ORDER BY medianPrice ASC")
	if err != nil {
		fmt.Println("failed to fetch from server:", err)
		internalError := http.StatusInternalServerError
		http.Error(w, err.Error(), internalError)
		return
	}
	defer rows.Close()

	jewelData := []db.DBJewelSnapshot{}
	for rows.Next() {
		s := db.DBJewelSnapshot{}
		err = rows.StructScan(&s)
		if err != nil {
			fmt.Printf("failed to scan struct\n")
			log.Fatal(err)
		}
		jewelData = append(jewelData, s)
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

func StartWebServer() {
	s, err := NewServer()
	if err != nil {
		fmt.Printf("failed to instantiate server")
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("./static"))

	http.HandleFunc("/", s.GetRoot)
	http.HandleFunc("/dump", s.GetDump)
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.ListenAndServe(":8080", nil)
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
