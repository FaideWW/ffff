package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const MAX_BACKOFFS = 6

var client = &http.Client{Timeout: 30 * time.Second}

func main() {

	l := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "development"
	}

	godotenv.Load(".env." + env + ".local")
	if env != "test" {
		godotenv.Load(".env.local")
	}

	godotenv.Load(".env." + env)
	godotenv.Load()

	nextCursor := os.Getenv("INITIAL_CHANGE_ID")
	if nextCursor == "" {
		l.Printf("No change id found in environment; fetching latest id from API\n")
		var err error
		nextCursor, err = GetLatestChangeId()
		if err != nil {
			log.Fatal(err)
		}
	}
	l.Printf("Starting change id: %s\n", nextCursor)

	// Init connection to the database
	pqCfg := PQConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASS"),
		Dbname:   os.Getenv("DB_NAME"),
		Host:     os.Getenv("DB_HOST"),
		Sslmode:  "verify-full",
	}

	db, err := DBConnect(&pqCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// test the connection
	var version string
	if err := db.QueryRow("select version()").Scan(&version); err != nil {
		log.Fatal(err)
	}
	l.Printf("DB connection established; version: %s\n", version)

	nextWaitMs := 0
	backoffs := 0

	for {
		url := "https://api.pathofexile.com/public-stash-tabs"
		if len(nextCursor) > 0 {
			url = url + "?id=" + nextCursor
		}

		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("Authorization", "Bearer "+os.Getenv("GGG_OAUTH_TOKEN"))
		req.Header.Add("User-Agent", os.Getenv("GGG_USERAGENT"))

		if err != nil {
			log.Fatal(err)
		}

		l.Println("Sending request")
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		rateLimitExceeded := false
		// Handle rate limit
		if resp.StatusCode == 429 {
			rateLimitExceeded = true
			retryS, err := strconv.Atoi(resp.Header.Get("Retry-After"))
			if err != nil {
				log.Fatal(err)
			}
			nextWaitMs = retryS * 1000
		} else if resp.StatusCode == 200 {
			nextWaitMs = 0
		}

		// decode rate limit policy
		rateLimitRules := strings.Split(resp.Header.Get("x-rate-limit-rules"), ",")

		for _, rule := range rateLimitRules {
			policyHeader := "x-rate-limit-" + rule
			policyValues := strings.Split(resp.Header.Get(policyHeader), ":")
			maxHits, err := strconv.Atoi(policyValues[0])
			if err != nil {
				log.Fatal(err)
			}
			periodS, err := strconv.Atoi(policyValues[1])
			if err != nil {
				log.Fatal(err)
			}

			ruleIntervalMs := (periodS * 1000) / maxHits

			if ruleIntervalMs > nextWaitMs {
				nextWaitMs = ruleIntervalMs
			}
		}

		nextCursor = resp.Header.Get("x-next-change-id")
		if nextCursor != "" {
			l.Printf("Next stash change id: %s\n", nextCursor)
		}
		if nextCursor == "" && nextWaitMs == 0 {
			// We've reached the end, pause the reader (if it hasn't been paused already
			l.Printf("No next change id\n")
			nextWaitMs = 60
		}

		decodeStart := time.Now()
		tabs, decodeErr := FindFFJewels(resp.Body, l)
		if decodeErr != nil && decodeErr != io.EOF {
			log.Fatal(decodeErr)
		}
		decodeEnd := time.Since(decodeStart)

		l.Printf("Response: processed %d stash tabs in %s\n", len(tabs), decodeEnd)

		// Slowly back off if we're at the front of the river
		if len(tabs) == 0 && !rateLimitExceeded {
			nextWaitMs = nextWaitMs * IntPow(2, backoffs)
			if backoffs < MAX_BACKOFFS {
				backoffs++
			}
		} else if len(tabs) > 0 {
			backoffs = 0
			dbStart := time.Now()
			ctx := context.TODO()
			// TODO: make this a goroutine? or if it's really slow, add a message broker here
			UpdateDb(ctx, db, tabs)
			dbEnd := time.Since(dbStart)
			l.Printf("Response: database updated in %s\n", dbEnd)
		}

		if nextWaitMs > 0 {
			waitDuration := time.Duration(nextWaitMs)*time.Millisecond - decodeEnd
			if waitDuration < 0 {
				waitDuration = 0
			}
			l.Printf("waiting %s...\n", waitDuration)
			time.Sleep(waitDuration)
		}
	}
}

func IntPow(n, m int) int {
	if m == 0 {
		return 1
	}
	result := n
	for i := 2; i <= m; i++ {
		result *= n
	}
	return result
}
