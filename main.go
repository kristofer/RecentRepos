package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	DB            *sql.DB
	GitHubService *GitHubService
}

type GitHubActivity struct {
	ID          int       `json:"id"`
	Date        time.Time `json:"date"`
	Repository  string    `json:"repository"`
	ActivityType string   `json:"activity_type"`
	Count       int       `json:"count"`
	URL         string    `json:"url"`
}

func main() {
	app := &App{
		GitHubService: NewGitHubService(),
	}
	
	// Initialize database
	if err := app.initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer app.DB.Close()

	// Set up routes
	r := mux.NewRouter()
	r.HandleFunc("/", app.indexHandler).Methods("GET")
	r.HandleFunc("/api/activity", app.getActivityHandler).Methods("GET")
	r.HandleFunc("/api/refresh", app.refreshActivityHandler).Methods("POST")
	r.HandleFunc("/api/status", app.statusHandler).Methods("GET")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func (app *App) initDB() error {
	var err error
	app.DB, err = sql.Open("sqlite3", "./activity.db")
	if err != nil {
		return err
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS github_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		repository TEXT NOT NULL,
		activity_type TEXT NOT NULL,
		count INTEGER DEFAULT 1,
		url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_date ON github_activity(date);
	CREATE INDEX IF NOT EXISTS idx_repo ON github_activity(repository);
	`

	_, err = app.DB.Exec(createTableSQL)
	return err
}

func (app *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func (app *App) getActivityHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := app.DB.Query(`
		SELECT id, date, repository, activity_type, count, COALESCE(url, '') as url
		FROM github_activity 
		ORDER BY date DESC 
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var activities []GitHubActivity
	for rows.Next() {
		var activity GitHubActivity
		var dateStr string
		err := rows.Scan(&activity.ID, &dateStr, &activity.Repository, &activity.ActivityType, &activity.Count, &activity.URL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		activity.Date, _ = time.Parse("2006-01-02", dateStr)
		activities = append(activities, activity)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
}

func (app *App) refreshActivityHandler(w http.ResponseWriter, r *http.Request) {
	// This will fetch data from GitHub API and store in database
	err := app.fetchGitHubActivity()
	if err != nil {
		http.Error(w, "Failed to refresh activity: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (app *App) fetchGitHubActivity() error {
	// Get GitHub username from environment or use default
	username := os.Getenv("GITHUB_USERNAME")
	if username == "" {
		username = "kristofer" // Default username
	}

	activities, err := app.GitHubService.FetchUserActivity(username)
	if err != nil {
		return fmt.Errorf("failed to fetch GitHub activity: %w", err)
	}

	// Clear existing data (optional - you might want to keep historical data)
	_, err = app.DB.Exec("DELETE FROM github_activity WHERE date >= date('now', '-30 days')")
	if err != nil {
		return fmt.Errorf("failed to clear old activity: %w", err)
	}

	// Insert new activity data
	for _, activity := range activities {
		_, err := app.DB.Exec(`
			INSERT OR REPLACE INTO github_activity (date, repository, activity_type, count, url)
			VALUES (?, ?, ?, ?, ?)
		`, activity.Date.Format("2006-01-02"), activity.Repository, activity.ActivityType, activity.Count, activity.URL)
		if err != nil {
			return fmt.Errorf("failed to insert activity: %w", err)
		}
	}

	return nil
}

func (app *App) statusHandler(w http.ResponseWriter, r *http.Request) {
	githubToken := os.Getenv("GITHUB_TOKEN")
	githubUsername := os.Getenv("GITHUB_USERNAME")
	if githubUsername == "" {
		githubUsername = "kristofer"
	}

	status := map[string]interface{}{
		"github_token_configured": githubToken != "",
		"github_username":         githubUsername,
		"database_connected":      app.DB != nil,
		"sample_mode":            githubToken == "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}