package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// URLStore manages URL storage
type URLStore struct {
	db *sql.DB
}

// NewURLStore creates a new URLStore
func NewURLStore(dbFile string) (*URLStore, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}
	
	// Create table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			short_code TEXT UNIQUE NOT NULL,
			original_url TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}
	
	return &URLStore{db: db}, nil
}

// Closes the database connection
func (s *URLStore) Close() error {
	return s.db.Close()
}

// Get retrieves a URL for a given short code
func (s *URLStore) Get(shortCode string) (string, bool) {
	var longURL string
	err := s.db.QueryRow("SELECT original_url FROM urls WHERE short_code = ?", shortCode).Scan(&longURL)
	if err != nil {
		return "", false
	}
	return longURL, true
}

// Set stores a URL with a generated short code
func (s *URLStore) Set(longURL string) (string, error) {
	// check if URL already exists
	var shortCode string
	err := s.db.QueryRow("SELECT short_code FROM urls WHERE original_url = ?", longURL).Scan(&shortCode)
	if err == nil {
		return shortCode, nil // URL already exists
	}

	// Generate new short code
	shortCode = generateShortCode(longURL)

	// Insert into database
	_, err = s.db.Exec("INSERT INTO urls (short_code, original_url) VALUES (?, ?)", shortCode, longURL)
	if err != nil {
		return "", err
	}

	return shortCode, nil
}

// This function turns a URL into a short code 
func generateShortCode(url string) string {
	// Create SHA256 has of URL
	hash := sha256.Sum256([]byte(data))

	// Convert hash to base64 string 
	encoded := base64.URLEncoding.EncodeToString(hash[:])

	// Take first 8 characters for the short code
	return encoded[:8]

}

var store *URLStore

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
	shortCode, err := store.Set(longURL)
	if err != nil {
		http.Error(w, "Error creating short URL", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	shortURL := fmt.Sprintf("%s/%s", os.Getenv("BASE_URL"), shortCode)

	fmt.Fprintf(w, `
		<html>
		<head><title>URL Shortened</title></head>
		<body>
			<h1>URL Shortened</h1>
			<p>Original URL: %s</p>
			<p>Short URL: <a href="%s">%s</a></p>
			<p><a href="/">Create another</a></p>
		</body>
		</html>
	`, longURL, shortURL, shortURL)
}

func main() {
	var err error
	// Initialize the URL store with a database file
	store, err = NewURLStore("urls.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	// Register route handlers
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/shorten", shortenHandler)

	// Start the server
	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}