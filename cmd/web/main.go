package main

import (
	"os"

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

func parseFlags(f *CliFlags) {
	// flag.BoolVar(&f.StartFromHead, "startFromHead", false, "whether to query the trade api for the river head and begin from there, or resume from the latest changeset in the `changesets` table")
}

func main() {
	loadEnv()
	f := CliFlags{}
	parseFlags(&f)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	StartWebServer(port)
}
