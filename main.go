package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"sync"
	"os"
)

// URLStore stores the mapping between short codes and URLS
type URLStore struct {
	urls map[string]string // short code -> long url
	mutex sync.RWMutex
}

// NewURLStore creates a new URLStore
func NewURLStore() *URLStore {
	return &URLStore{
		urls: make(map[string]string),
	}
}

// Get retrieves a URL for a given short code
func (s *URLStore) Get(shortCode string) (string, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	url, exists := s.urls[shortCode]
	return url, exists
}

// Set stores a URL with a generated short code
func (s *URLStore) Set(longURL string) string {
	shortCode := generateShortCode(longURL)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if this URL already has a short code
	for code, url := range s.urls {
		if url == longURL {
			return code
		}
	}

	s.urls[shortCode] = longURL
	return shortCode
}

// This function turns a URL into a short code 
func generateShortCode(url string) string {
	// Create SHA256 has of URL
	hash := sha256.Sum256([]byte(url))

	// Convert hash to base64 string 
	encoded := base64.URLEncoding.EncodeToString(hash[:])

	// Take first 8 characters for the short code
	return encoded[:8]

}

var store = NewURLStore()

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Check if it is a redirect request
	path := r.URL.Path[1:] // Remove the leading slash
	if path != "" {
		longURL, exists := store.Get(path)
		if exists {
			http.Redirect(w, r, longURL, http.StatusFound)
			return
		}
		http.NotFound(w, r)
		return
	}

	// Display home page
	fmt.Fprintf(w, `
		<html>
		<head><title>URL Shortener</title></head>
		<body>
			<h1>URL Shortener</h1>
			<form action="/shorten" method="post">
				<input type="text" name="url" placeholder="Enter a URL">
				<input type="submit" value="shorten">
			</form>
		</body>
		</html>
	`)
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
	shortCode := store.Set(longURL)
	shortURL := fmt.Sprintf("%s/%s", os.Getenv("BASE_URL"), shortCode)

	// TODO: Store mapping in a db

	fmt.Fprintf(w, `
		<html>
		<head><title>URL Shortened</title></head>
		<body>
			<h1>URL Shortened</h1>
			<p>Original URL: %s"</p>
			<p>Short URL: <a href="%s">%s</a></p>
			<p><a href="/">Create another</a></p>
		</body>
		</html>
	`, longURL, shortURL, shortURL)
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