package handlers

import (
	"encoding/json"
	"google-monitoring/cities"
	"net/http"
)

func GetCities() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		citiesList := cities.GetCities()

		citiesJSON, err := json.Marshal(citiesList)
		if err != nil {
			http.Error(w, "Failed to marshal cities list", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(citiesJSON)
	}
}