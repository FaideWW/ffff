package psapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"

	db "github.com/faideww/ffff/internal/db"
	"github.com/faideww/ffff/internal/poeninja"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CliFlags struct {
	StartFromHead bool
}

const MAX_BACKOFFS = 6
const POENINJA_POLL_RATE = 60 // Every 60 iterations (~30s)

func ConsumeRiver(f *CliFlags) {
	l := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// Init connection to the database
	dbHandle, err := db.DBConnect(os.Getenv("PG_DB_CONNSTR"))
	if err != nil {
		log.Panic(err)
	}
	defer dbHandle.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	nextCursor := os.Getenv("INITIAL_CHANGE_ID")
	if nextCursor == "" {
		l.Printf("No change id found in environment\n")
		l.Printf("args: %+v\n", f)
		if f.StartFromHead {
			l.Printf("fetching latest id from API\n")
			nextCursor, err = poeninja.GetLatestPSChangeId(client)
			if err != nil {
				log.Panic(err)
			}
		} else {
			l.Printf("resuming from last changeset id\n")
			row := dbHandle.QueryRow(context.Background(), "SELECT nextChangeId FROM changesets ORDER BY processedAt DESC LIMIT 1")
			err = row.Scan(&nextCursor)
			if err != nil {

				if errors.Is(err, pgx.ErrNoRows) {
					l.Printf("no changesets found to resume from; exiting\n")
				}
				log.Panic(err)
			}
		}
	} else if f.StartFromHead {
		if err != nil {
			log.Panic(errors.New("both startFromHead and INITIAL_CHANGE_ID were set, this is probably not intended. exiting"))
		}
	}
	l.Printf("Starting change id: %s\n", nextCursor)

	nextWaitMs := 0
	backoffs := 0
	poeNinjaPollIndex := 0

	for {
		func() {
			url := "https://api.pathofexile.com/public-stash-tabs"
			if len(nextCursor) > 0 {
				url = url + "?id=" + nextCursor
			}

			req, err := http.NewRequest("GET", url, nil)
			req.Header.Add("Authorization", "Bearer "+os.Getenv("GGG_OAUTH_TOKEN"))
			req.Header.Add("User-Agent", os.Getenv("GGG_USERAGENT"))

			if err != nil {
				log.Panic(err)
			}

			l.Println("Polling psapi...")
			resp, err := client.Do(req)
			reqHandleStart := time.Now()
			if err != nil {
				l.Printf("request errored out: %s\n", err)
				if resp != nil {
					l.Printf("Request failed: %s\n", resp.Status)
					for k, v := range resp.Header {
						l.Printf("  %s=%s\n", k, v)
					}
				}
				log.Panic(err)
			}
			defer resp.Body.Close()

			rateLimitExceeded := false
			// Handle rate limit
			switch resp.StatusCode {
			case 429:
				l.Printf("psapi returned 429 (Too Many Requests)\n")
				rateLimitExceeded = true
				retryS, retryErr := strconv.Atoi(resp.Header.Get("Retry-After"))
				if retryErr != nil {
					l.Printf("failed to parse Retry-After header; panicing\n")
					l.Printf("Headers:\n%+v\n", resp.Header)
					log.Panic(retryErr)
				}
				nextWaitMs = retryS * 1000
			case 200:
				nextWaitMs = 0
			}

			// decode rate limit policy
			rateLimitRules := strings.Split(resp.Header.Get("x-rate-limit-rules"), ",")

			for _, rule := range rateLimitRules {
				policyHeader := "x-rate-limit-" + rule
				policyValues := strings.Split(resp.Header.Get(policyHeader), ":")
				maxHits, maxHitsErr := strconv.Atoi(policyValues[0])
				if maxHitsErr != nil {
					l.Printf("Failed to decode rate-limit header - %s\n", maxHitsErr)
					l.Printf("Response status: %s\n", resp.Status)
					l.Printf("Headers:\n%+v\n", resp.Header)
					respDump, _ := httputil.DumpResponse(resp, false)
					l.Printf("Full response: \n%s\n", string(respDump))
					log.Panic(maxHitsErr)
				}
				periodS, periodErr := strconv.Atoi(policyValues[1])
				if periodErr != nil {
					l.Printf("Failed to decode rate-limit header - %s\n", periodErr)
					l.Printf("Headers:\n%+v\n", resp.Header)
					respDump, _ := httputil.DumpResponse(resp, false)
					l.Printf("Full response: \n%+v\n", respDump)
					log.Panic(periodErr)
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

			type HeadResponse struct {
				id   string
				skip bool
				err  error
			}
			headCh := make(chan HeadResponse, 1)
			go func(ch chan HeadResponse) {
				if poeNinjaPollIndex == 0 {
					res, ninjaErr := poeninja.GetLatestPSChangeId(client)
					ch <- HeadResponse{res, false, ninjaErr}
				} else {
					ch <- HeadResponse{"", true, nil}
				}
			}(headCh)

			decodeStart := time.Now()
			tabs, decodeErr := FindFFJewels(resp.Body, l, currentCursor)
			if decodeErr != nil && decodeErr != io.EOF {
				log.Panic(decodeErr)
			}
			decodeEnd := time.Since(decodeStart)

			l.Printf("Response: processed %d stash tabs in %s\n", len(tabs), decodeEnd)
			ctx := context.TODO()

			// Slowly back off if we're at the front of the river
			if len(tabs) == 0 && !rateLimitExceeded {
				nextWaitMs = nextWaitMs * IntPow(2, backoffs)
				if backoffs < MAX_BACKOFFS {
					backoffs++
				}
			} else if len(tabs) > 0 {
				backoffs = 0
				dbStart := time.Now()
				// TODO: make this a goroutine? or if it's really slow, add a message broker here
				err = UpdateDb(ctx, dbHandle, tabs)
				if err != nil {
					log.Panic(err)
				}
				dbEnd := time.Since(dbStart)
				l.Printf("Response: database updated in %s\n", dbEnd)
			}

			reqHandleEnd := time.Since(reqHandleStart)

			headRes := <-headCh
			if headRes.err != nil {
				l.Printf("could not fetch latest change id: %s\n", headRes.err)
				log.Panic(headRes.err)
			}

			var pgDrift pgtype.Int4
			if !headRes.skip {

				// calculate drift from the river head
				drift, driftErr := CalculateRiverDrift(headRes.id, currentCursor)
				if driftErr != nil {
					l.Printf("failed to calculate river drift: %s\n", driftErr)
					// log.Panic(driftErr)
				}
				pgDrift.Int32 = int32(drift)
				pgDrift.Valid = true
			} else {
				pgDrift.Int32 = 0
				pgDrift.Valid = false
			}
			if len(tabs) > 0 {
				c := db.DBChangeset{
					ChangeId:     currentCursor,
					NextChangeId: nextCursor,
					StashCount:   len(tabs),
					// TODO: make sure all timestamps are consistent
					ProcessedAt:   decodeStart,
					TimeTakenMs:   reqHandleEnd.Milliseconds(),
					DriftFromHead: pgDrift,
				}

				_, err = dbHandle.Exec(ctx, "INSERT INTO changesets(changeId,nextChangeId,stashCount,processedAt,timeTaken,driftFromHead) VALUES (@changeId,@nextChangeId,@stashCount,@processedAt,@timeTaken,@driftFromHead)", pgx.NamedArgs{
					"changeId":      c.ChangeId,
					"nextChangeId":  c.NextChangeId,
					"stashCount":    c.StashCount,
					"processedAt":   c.ProcessedAt,
					"timeTaken":     c.TimeTakenMs,
					"driftFromHead": c.DriftFromHead,
				})
				if err != nil {
					log.Panic(err)
				}
			}

			if nextWaitMs > 0 {
				waitDuration := time.Duration(nextWaitMs)*time.Millisecond - reqHandleEnd
				if waitDuration < 0 {
					waitDuration = 0
				}
				l.Printf("waiting %s...\n", waitDuration)
				time.Sleep(waitDuration)
			}
			poeNinjaPollIndex = (poeNinjaPollIndex + 1) % POENINJA_POLL_RATE
		}()
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

func CalculateRiverDrift(head string, current string) (int, error) {
	headShards := strings.Split(head, "-")
	currentShards := strings.Split(current, "-")

	if len(headShards) != len(currentShards) {
		fmt.Printf("head: %s\n", head)
		fmt.Printf("current: %s\n", current)
		return 0, errors.New("change ids have different number of shards")
	}

	headInts := make([]int64, len(headShards))
	for i, s := range headShards {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, err
		}
		headInts[i] = n
	}

	currentInts := make([]int64, len(currentShards))
	for i, s := range currentShards {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, err
		}
		currentInts[i] = n
	}

	sum := 0
	for i := 0; i < len(headInts); i++ {
		sum += int(headInts[i] - currentInts[i])
	}

	return sum, nil
}
