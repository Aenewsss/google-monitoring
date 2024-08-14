package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI   string
	SerpAPIKey string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	config := &Config{
		MongoURI:   os.Getenv("MONGODB_URI"),
		SerpAPIKey: os.Getenv("SERP_API_KEY"),
	}

	return config
}