package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
)

// This function turns a URL into a short code 
func generateShortCode(url string) string {
	// Create SHA256 has of URL
	hash := sha256.Sum256([]byte(url))

	// Convert hash to base64 string 
	encoded := base64.URLEncoding.EncodeToString(hash[:])

	// Take first 8 characters for the short code
	shortCode := encoded[:8]

	return shortCode
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "URL Shortener Service - Home Page")
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not parse form", http.StatusBadRequest)
		return
	}

	// Get URL from form
	longURL := r.FormValue("url")
	if longURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Generate short code
	shortCode := generateShortCode(longURL)

	// TODO: Store mapping in a db

	fmt.Fprintf(w, "Short URL: http://localhost:8080/%s", shortCode)
}

func main() {
	// Register route handlers
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/shorten", shortenHandler)

	// Start the server
	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}