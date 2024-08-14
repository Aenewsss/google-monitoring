package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	g "github.com/serpapi/google-search-results-golang"

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

type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Link    string `json:"link"`
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

		response, err := GoogleApiSearch(adsOrOrganicJSON, req.Query)
		if err != nil {
			http.Error(w, "Failed to get google api response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	}
}

func GoogleApiSearch(adsOrOrganicJSON []byte, query string) ([]byte, error) {
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

	for i, result := range linkResults {
		linkSite := result.Link
		fmt.Printf("\n#%d: %s\n", i+1, linkSite)

		resp, err := svc.Cse.List().Cx(cx).Q(query).LinkSite(linkSite).Do()
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
		}
	}

	resultsJSON, err := json.Marshal(searchResults)
	if err != nil {
		return nil, err
	}

	return resultsJSON, nil
}