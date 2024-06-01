package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

type TMDBMovie struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	Rating      float32 `json:"vote_average"`
	ReleaseDate string  `json:"release_date"`
	PosterPath  string  `json:"poster_path"`
}

type TMDBSearchResponse struct {
	Results []TMDBMovie `json:"results"`
}

func HandleMovieQuery(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf(
		"https://api.themoviedb.org/3/search/movie?include_adult=false&language=en-US&page=1&%s",
		r.URL.RawQuery,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.Header.Add("authorization", "Bearer "+os.Getenv("TMDB_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
