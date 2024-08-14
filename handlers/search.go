package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	g "github.com/serpapi/google-search-results-golang"

	config "google-monitoring/config"
)

type SearchRequest struct {
	City   string `json:"city"`
	Query  string `json:"query"`
	Device string `json:"device"`
}

func SearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		ads, ok := results["ads"]
		if !ok {
			http.Error(w, "'ads' field not found in search results", http.StatusNotFound)
			return
		}

		adsJSON, err := json.Marshal(ads)
		if err != nil {
			http.Error(w, "Failed to convert 'ads' to JSON", http.StatusInternalServerError)
			return
		}

		fmt.Println(string(adsJSON))

		w.Header().Set("Content-Type", "application/json")
		w.Write(adsJSON)
	}
}
