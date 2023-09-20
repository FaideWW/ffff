package main

import (
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

func main() {
	loadEnv()
	psapi.ConsumeRiver()
}
