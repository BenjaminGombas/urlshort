package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			hits INTEGER DEFAULT 0
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

	// Wrap in a transaction to increment hits atomically
	tx, err := s.db.Begin()
	if err != nil {
		log.Println("Transaction error:", err)
		return "", false
	}

	err = tx.QueryRow("SELECT original_url FROM urls WHERE short_code = ?", shortCode).Scan(&longURL)
	if err != nil {
		tx.Rollback()
		return "", false
	}

	_, err = tx.Exec("UPDATE urls SET hits = hits + 1 WHERE short_code = ?", shortCode)
	if err != nil {
		tx.Rollback()
		log.Println("Update error:", err)
		return "", false
	}

	if err = tx.Commit(); err != nil {
		log.Println("Commit error:", err)
		return "", false
	}

	return longURL, true
}

// Set stores a URL with a generated short code
func (s *URLStore) Set(longURL string) (string, error) {
	// Generate new short code
	shortCode := generateShortCode(longURL)

	// Insert into database
	_, err := s.db.Exec("INSERT INTO urls (short_code, original_url) VALUES (?, ?)", shortCode, longURL)
	if err != nil {
		return "", err
	}

	return shortCode, nil
}

// This function returns hit stats for a short code
func (s *URLStore) GetStats(shortCode string) (int, error) {
	var hits int
	err := s.db.QueryRow("SELECT hits FROM urls WHERE short_code = ?", shortCode).Scan(&hits)
	return hits, err
}

// This function turns a URL into a short code
func generateShortCode(url string) string {
	// Date will be used later to create a new code for the same URL for tracking purposes
	data := url + time.Now().String()
	// Create SHA256 has of URL
	hash := sha256.Sum256([]byte(data))

	// Convert hash to base64 string
	encoded := base64.URLEncoding.EncodeToString(hash[:])

	// Take first 8 characters for the short code
	return encoded[:8]

}

// This function checks if a string is a valid URL
func validateURL(rawURL string) (string, error) {

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	// Check for minimum required parts
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("URL missing scheme or host")
	}

	// Check for valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https")
	}

	// Check for valid host (at least one dot and no spaces)
	if !strings.Contains(parsedURL.Host, ".") || strings.Contains(parsedURL.Host, " ") {
		return "", fmt.Errorf("invalid host in URL")
	}

	// Return the normalized URL
	return parsedURL.String(), nil
}

var (
	store     *URLStore
	templates = template.Must(template.ParseGlob("templates/*.html"))
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Check if it is a redirect request
	path := r.URL.Path[1:] // Remove the leading slash
	if path != "" {
		if path == "favicon.ico" {
			http.NotFound(w, r)
			return
		}
		longURL, exists := store.Get(path)
		if exists {
			http.Redirect(w, r, longURL, http.StatusFound)
			return
		}
		// Not found - show error
		data := struct {
			Error string
		}{
			Error: "Short URL not found",
		}
		templates.ExecuteTemplate(w, "home.html", data)
		return
	}

	// Display home page
	templates.ExecuteTemplate(w, "home.html", nil)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		data := struct {
			Error string
		}{
			Error: "Could not parse form",
		}
		templates.ExecuteTemplate(w, "home.html", data)
		return
	}

	// Get URL from form
	longURL := r.FormValue("url")
	if longURL == "" {
		data := struct {
			Error string
		}{
			Error: "URL is required",
		}
		templates.ExecuteTemplate(w, "home.html", data)
		return
	}

	// Validate the URL
	validatedURL, err := validateURL(longURL)
	if err != nil {
		data := struct {
			Error string
		}{
			Error: "Invalid URL: " + err.Error(),
		}
		templates.ExecuteTemplate(w, "home.html", data)
		return
	}

	// Generate short code
	shortCode, err := store.Set(validatedURL)
	if err != nil {
		data := struct {
			Error string
		}{
			Error: "Error creating short URL: " + err.Error(),
		}
		templates.ExecuteTemplate(w, "home.html", data)
		return
	}

	shortURL := fmt.Sprintf("%s/%s", os.Getenv("BASE_URL"), shortCode)

	data := struct {
		LongURL  string
		ShortURL string
	}{
		LongURL:  longURL,
		ShortURL: shortURL,
	}

	templates.ExecuteTemplate(w, "result.html", data)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.URL.Query().Get("code")
	if shortCode == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	hits, err := store.GetStats(shortCode)
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, "Short URL /%s has been accessed %d times", shortCode, hits)
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
	http.HandleFunc("/stats", statsHandler)

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
