package main

import (
	"log"

	"github.com/faideww/ffff/internal/stats"
)

func main() {
	err := stats.AggregateStats(nil, nil)

	if err != nil {
		log.Fatal(err)
	}
}
