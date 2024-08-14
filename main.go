package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"google-monitoring/config"

	"google-monitoring/handlers"
)

func main() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(config.LoadConfig().MongoURI))
	if err != nil {
		panic(err)
	}

	defer client.Disconnect(context.TODO())

	err = client.Ping(context.TODO(), readpref.Primary())
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected to MongoDB!")

	http.HandleFunc("/cities", handlers.GetCities())
	http.HandleFunc("/search", handlers.SearchHandler())

	http.ListenAndServe(":8080", nil)

	log.Fatal(http.ListenAndServe(":8080", nil))
}