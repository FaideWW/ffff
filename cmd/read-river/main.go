package main

import (
	"flag"
	"os"

	psapi "github.com/faideww/ffff/internal/psapi"
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

func parseFlags(f *psapi.CliFlags) {
	flag.BoolVar(&f.StartFromHead, "startFromHead", false, "whether to query the trade api for the river head and begin from there, or resume from the latest changeset in the `changesets` table")
}

func main() {
	loadEnv()
	f := psapi.CliFlags{}
	parseFlags(&f)
	psapi.ConsumeRiver(&f)
}
