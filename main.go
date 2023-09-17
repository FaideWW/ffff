package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

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

	nextCursor := ""
	nextWait := 0

	buf := make([]byte, 1024*500) // 500KB buffer

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

		client := http.Client{}
		l.Println("Sending request")
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		// Handle rate limit
		if resp.StatusCode == 429 {
			nextWait, err = strconv.Atoi(resp.Header.Get("Retry-After"))
			if err != nil {
				log.Fatal(err)
			}
		} else if resp.StatusCode == 200 {
			nextWait = 0
		}

		l.Printf("Response: %s\n", resp.Status)
		l.Printf("Response: x-rate-limit-ip-state: %s\n", resp.Header.Get("x-rate-limit-ip-state"))
		l.Printf("Response: x-next-change-id: %s\n", resp.Header.Get("x-next-change-id"))

		nextCursor = resp.Header.Get("x-next-change-id")
		if nextCursor == "" && nextWait == 0 {
			// We've reached the end, pause the reader (if it hasn't been paused already
			nextWait = 60
		}

		// TODO: implement json decoder

		bytesRead := 0
		chunkNum := 0

		for {
			n, err := resp.Body.Read(buf)
			bytesRead += n

			if err == io.EOF {
				break
			}

			if err != nil {
				log.Fatal(err)
			}

			// writeErr := os.WriteFile("data/chunk-"+strconv.Itoa(chunkNum)+".txt", buf[0:bytesRead], 0644)
			// if writeErr != nil {
			// 	log.Fatal(writeErr)
			// }
			chunkNum++
		}

		l.Printf("Response: bytes read: %d from %d chunks\n", bytesRead, chunkNum)

		if nextWait > 0 {
			l.Printf("Rate limit reached or caught up to river; waiting %d seconds...\n", nextWait)
			time.Sleep(time.Duration(nextWait) * time.Second)
		}
	}

}
