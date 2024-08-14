package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"sync"

	g "github.com/serpapi/google-search-results-golang"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	config "google-monitoring/config"

	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/googleapi/transport"
)

type AdResult struct {
	Link string `json:"link"`
}

type SearchRequest struct {
	City   string `json:"city"`
	Query  string `json:"query"`
	Device string `json:"device"`
}

type TenCitiesSearchRequest struct {
	Cities []string `json:"cities"`
	Query  string   `json:"query"`
	Device string   `json:"device"`
	Email  string   `json:"email"`
}

type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Link    string `json:"link"`
}

func SearchHandler(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			collection := client.Database(config.LoadConfig().DbName).Collection("searches")

			filter := bson.M{}
			opts := options.Find()

			cursor, err := collection.Find(context.TODO(), filter, opts)
			if err != nil {
				http.Error(w, "Failed to retrieve search results", http.StatusInternalServerError)
				return
			}
			defer cursor.Close(context.TODO())

			var searchResults []SearchResult
			if err := cursor.All(context.TODO(), &searchResults); err != nil {
				http.Error(w, "Failed to decode search results", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(searchResults)
			return
		}

		if r.Method == "POST" {
			var req SearchRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request payload", http.StatusBadRequest)
				return
			}

			parameter := map[string]string{
				"engine":        "google",
				"location":      req.City,
				"q":             req.Query,
				"google_domain": "google.com.br",
				"gl":            "br",
				"hl":            "pt-br",
				"device":        req.Device,
			}

			apiKey := config.LoadConfig().SerpAPIKey

			search := g.NewGoogleSearch(parameter, apiKey)

			results, err := search.GetJSON()
			if err != nil {
				http.Error(w, "Failed to get search results", http.StatusInternalServerError)
				return
			}

			var adsOrOrganic interface{}
			var ok bool

			adsOrOrganic, ok = results["ads"]

			if !ok {
				adsOrOrganic, ok = results["organic_results"]
				if !ok {
					http.Error(w, "'ads' or 'organic_results' field not found in search results", http.StatusNotFound)
					return
				}
			}

			adsOrOrganicJSON, err := json.Marshal(adsOrOrganic)
			if err != nil {
				http.Error(w, "Failed to convert 'ads' or 'organic_results' to JSON", http.StatusInternalServerError)
				return
			}

			response, err := GoogleApiSearch(client, adsOrOrganicJSON, req.Query)
			if err != nil {
				http.Error(w, "Failed to get google api response", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
		}
	}
}

func TenCitiesSearchHandler(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req TenCitiesSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		if len(req.Cities) != 10 {
			http.Error(w, "Exactly 10 cities must be provided", http.StatusBadRequest)
			return
		}

		apiKey := config.LoadConfig().SerpAPIKey
		mongoCollection := client.Database(config.LoadConfig().DbName).Collection("searches")

		var allResults []SearchResult
		var emailBody bytes.Buffer
		var mu sync.Mutex
		var wg sync.WaitGroup

		cityChan := make(chan string, len(req.Cities))
		for _, city := range req.Cities {
			cityChan <- city
		}
		close(cityChan)

		numWorkers := 3 

		searchLimit := 20
		searchCounter := 0

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for city := range cityChan {
					mu.Lock()
					if searchCounter >= searchLimit {
						mu.Unlock()
						fmt.Printf("Search limit of %d reached, skipping further searches.\n", searchLimit)
						break
					}
					searchCounter++
					mu.Unlock()

					parameter := map[string]string{
						"engine":        "google",
						"location":      city,
						"q":             req.Query,
						"google_domain": "google.com.br",
						"gl":            "br",
						"hl":            "pt-br",
						"device":        req.Device,
					}

					search := g.NewGoogleSearch(parameter, apiKey)
					results, err := search.GetJSON()
					if err != nil {
						fmt.Printf("Failed to get search results for city %s: %v\n", city, err)
						continue
					}

					adsOrOrganic, ok := results["ads"]
					if !ok {
						adsOrOrganic, ok = results["organic_results"]
						if !ok {
							fmt.Printf("'ads' or 'organic_results' field not found for city %s\n", city)
							continue
						}
					}

					adsOrOrganicJSON, err := json.Marshal(adsOrOrganic)
					if err != nil {
						fmt.Printf("Failed to convert 'ads' or 'organic_results' to JSON for city %s: %v\n", city, err)
						continue
					}

					siteData, err := GoogleApiSearch(client, adsOrOrganicJSON, req.Query)
					if err != nil {
						fmt.Printf("Failed to get google api response for city %s: %v\n", city, err)
						continue
					}

					mu.Lock()
					var searchResults []SearchResult
					if err := json.Unmarshal(siteData, &searchResults); err != nil {
						fmt.Printf("Failed to unmarshal search results for city %s: %v\n", city, err)
						mu.Unlock()
						continue
					}

					for _, result := range searchResults {
						_, err := mongoCollection.InsertOne(context.TODO(), result)
						if err != nil {
							fmt.Printf("Failed to insert search result into MongoDB for city %s: %s\n", city, err.Error())
						}

						emailBody.WriteString(fmt.Sprintf("Cidade: %s\nTítulo: %s\nDesrição: %s\nLink: %s\n\n", city, result.Title, result.Snippet, result.Link))
					}

					allResults = append(allResults, searchResults...)
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		go func() {
			if err := SendEmail(req.Email, emailBody.String()); err != nil {
				fmt.Printf("Failed to send email: %v\n", err)
			}
		}()

		// Return the combined results as a JSON response
		resultsJSON, err := json.Marshal(allResults)
		if err != nil {
			http.Error(w, "Failed to marshal combined search results", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resultsJSON)
	}
}

func GoogleApiSearch(mongoClient *mongo.Client, adsOrOrganicJSON []byte, query string) ([]byte, error) {
	apiKey := config.LoadConfig().CustomSearchAPIKey
	cx := config.LoadConfig().SearchEngineID

	client := &http.Client{Transport: &transport.APIKey{Key: apiKey}}

	svc, err := customsearch.New(client)
	if err != nil {
		return nil, err
	}

	var linkResults []AdResult
	if err := json.Unmarshal(adsOrOrganicJSON, &linkResults); err != nil {
		fmt.Printf("Failed to unmarshal ads JSON: %s\n", err.Error())
		return nil, err
	}

	var searchResults []SearchResult

	mongoCollection := mongoClient.Database(config.LoadConfig().DbName).Collection("searches")

	for _, result := range linkResults {
		linkSite := result.Link

		resp, err := svc.Cse.List().Cx(cx).Q(query).SiteSearch(linkSite).Do()
		if err != nil {
			fmt.Printf(err.Error())
		}

		if len(resp.Items) > 0 {
			item := resp.Items[0]

			searchResult := SearchResult{
				Title:   item.Title,
				Snippet: item.Snippet,
				Link:    item.Link,
			}

			searchResults = append(searchResults, searchResult)

			_, err := mongoCollection.InsertOne(context.TODO(), searchResult)
			if err != nil {
				fmt.Printf("Failed to insert search result into MongoDB: %s\n", err.Error())
			}
		}
	}

	resultsJSON, err := json.Marshal(searchResults)
	if err != nil {
		return nil, err
	}

	return resultsJSON, nil
}

func SendEmail(to, body string) error {
	from := config.LoadConfig().MailFrom
	password := config.LoadConfig().MailPassword

	// Set up authentication information.
	auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")

	// Create the email message.
	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: Monitoramente Brand | Resultados\n\n" +
		body

	// Send the email.
	err := smtp.SendMail("smtp.gmail.com:587", auth, from, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
