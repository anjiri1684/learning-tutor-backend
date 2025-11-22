package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func Config(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Warning: .env file not found, reading from system environment variables")
	}

	return os.Getenv(key)
}