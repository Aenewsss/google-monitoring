package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI           string
	SerpAPIKey         string
	CustomSearchAPIKey string
	SearchEngineID     string
	DbName             string
	MailFrom           string
	MailPassword       string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	config := &Config{
		MongoURI:           os.Getenv("MONGODB_URI"),
		SerpAPIKey:         os.Getenv("SERP_API_KEY"),
		CustomSearchAPIKey: os.Getenv("CUSTOM_SEARCH_API_KEY"),
		SearchEngineID:     os.Getenv("SEARCH_ENGINE_ID"),
		DbName:             os.Getenv("MONGODB_NAME"),
		MailFrom:           os.Getenv("MAIL_FROM"),
		MailPassword:       os.Getenv("MAIL_PASSWORD"),
	}

	return config
}
