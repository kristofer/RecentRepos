package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	DB            *sql.DB
	GitHubService *GitHubService
}

type GitHubActivity struct {
	ID           int       `json:"id"`
	Date         time.Time `json:"date"`
	Repository   string    `json:"repository"`
	ActivityType string    `json:"activity_type"`
	Count        int       `json:"count"`
	URL          string    `json:"url"`
}

// Handler for /api/commits: returns last 6 months of commits grouped by repo, ordered by most recent commit per repo
func (app *App) getCommitsHandler(w http.ResponseWriter, r *http.Request) {
	// Get pagination parameters
	page := 1
	limit := 100
	
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	sixMonthsAgo := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	
	// First, get total count of repositories with commits
	var totalRepos int
	err := app.DB.QueryRow(`
		SELECT COUNT(DISTINCT repository) 
		FROM github_activity 
		WHERE activity_type = 'commit' AND date >= ?
	`, sixMonthsAgo).Scan(&totalRepos)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get all commits data first, then group and paginate
	rows, err := app.DB.Query(`
		SELECT repository, date, url, count, activity_type
		FROM github_activity
		WHERE activity_type = 'commit' AND date >= ?
		ORDER BY date DESC, repository
	`, sixMonthsAgo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Group commits by repo
	repoCommits := make(map[string][]GitHubActivity)
	for rows.Next() {
		var repo, dateStr, url, activityType string
		var count int
		err := rows.Scan(&repo, &dateStr, &url, &count, &activityType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		date, _ := time.Parse("2006-01-02", dateStr)
		activity := GitHubActivity{
			Date:         date,
			Repository:   repo,
			ActivityType: activityType,
			Count:        count,
			URL:          url,
		}
		repoCommits[repo] = append(repoCommits[repo], activity)
	}

	// Prepare ordered list of repos by most recent commit
	type RepoGroup struct {
		Repository string           `json:"repository"`
		Commits    []GitHubActivity `json:"commits"`
		LatestDate time.Time        `json:"latest_date"`
	}
	var allRepoGroups []RepoGroup
	for repo, commits := range repoCommits {
		if len(commits) > 0 {
			latest := commits[0].Date
			allRepoGroups = append(allRepoGroups, RepoGroup{
				Repository: repo,
				Commits:    commits,
				LatestDate: latest,
			})
		}
	}
	
	// Sort repos by most recent commit date (descending)
	sort.Slice(allRepoGroups, func(i, j int) bool {
		return allRepoGroups[i].LatestDate.After(allRepoGroups[j].LatestDate)
	})

	// Apply pagination
	start := (page - 1) * limit
	end := start + limit
	if start > len(allRepoGroups) {
		start = len(allRepoGroups)
	}
	if end > len(allRepoGroups) {
		end = len(allRepoGroups)
	}

	paginatedRepoGroups := allRepoGroups[start:end]

	// Prepare response with pagination metadata
	response := map[string]interface{}{
		"data": paginatedRepoGroups,
		"pagination": map[string]interface{}{
			"page":         page,
			"limit":        limit,
			"total":        len(allRepoGroups),
			"total_pages":  (len(allRepoGroups) + limit - 1) / limit,
			"has_next":     end < len(allRepoGroups),
			"has_prev":     page > 1,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
		"sample_mode":             githubToken == "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func main() {
	app := &App{
		GitHubService: NewGitHubService(),
	}

	// Initialize database
	if err := app.initDB(); err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}
	defer app.DB.Close()

	// Set up routes
	r := http.NewServeMux()
	r.HandleFunc("/", app.indexHandler)
	r.HandleFunc("/api/activity", app.getActivityHandler)
	r.HandleFunc("/api/commits", app.getCommitsHandler)
	r.HandleFunc("/api/refresh", app.refreshActivityHandler)
	r.HandleFunc("/api/status", app.statusHandler)

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		fmt.Println("Server error:", err)
	}
}
