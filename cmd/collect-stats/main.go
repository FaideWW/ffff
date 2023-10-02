package main

import (
	"log"
	"os"

	"github.com/faideww/ffff/internal/stats"
	"github.com/joho/godotenv"
)

func loadEnv() {
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

}

func main() {
	loadEnv()
	err := stats.AggregateStats(nil, nil)

	if err != nil {
		log.Fatal(err)
	}
}
