package psapi

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	db "github.com/faideww/ffff/internal/db"
)

const MAX_BACKOFFS = 6

func ConsumeRiver() {
	l := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	client := &http.Client{Timeout: 30 * time.Second}
	nextCursor := os.Getenv("INITIAL_CHANGE_ID")
	if nextCursor == "" {
		l.Printf("No change id found in environment; fetching latest id from API\n")
		var err error
		nextCursor, err = GetLatestChangeId(client)
		if err != nil {
			log.Fatal(err)
		}
	}
	l.Printf("Starting change id: %s\n", nextCursor)

	// Init connection to the database
	pqCfg := db.PQConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASS"),
		Dbname:   os.Getenv("DB_NAME"),
		Host:     os.Getenv("DB_HOST"),
		Sslmode:  "verify-full",
	}

	db, err := db.DBConnect(&pqCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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
		reqHandleStart := time.Now()
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

		currentCursor := nextCursor
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
		tabs, decodeErr := FindFFJewels(resp.Body, l, currentCursor)
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
			err := UpdateDb(ctx, db, tabs)
			if err != nil {
				log.Fatal(err)
			}
			dbEnd := time.Since(dbStart)
			l.Printf("Response: database updated in %s\n", dbEnd)
		}

		reqHandleEnd := time.Since(reqHandleStart)

		if nextWaitMs > 0 {
			waitDuration := time.Duration(nextWaitMs)*time.Millisecond - reqHandleEnd
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
