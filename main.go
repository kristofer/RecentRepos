package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
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
	GitHubID     string    `json:"github_id"` // Unique identifier from GitHub (SHA for commits, number for PRs/issues)
}

type PRComment struct {
	ID         int       `json:"id"`
	Repository string    `json:"repository"`
	PRNumber   int       `json:"pr_number"`
	PRTitle    string    `json:"pr_title"`
	Author     string    `json:"author"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	PRURL      string    `json:"pr_url"`
	CommentURL string    `json:"comment_url"`
}

type ProjectEntry struct {
	Repository    string      `json:"repository"`
	LatestDate    time.Time   `json:"latest_date"`
	TotalCommits  int         `json:"total_commits"`
	ActivityTypes []string    `json:"activity_types"`
	RecentComments []PRComment `json:"recent_comments"`
	URL           string      `json:"url"`
}

type BlogEntry struct {
	Repository   string           `json:"repository"`
	LatestDate   time.Time        `json:"latest_date"`
	URL          string           `json:"url"`
	PullRequests []GitHubActivity `json:"pull_requests"`
	Issues       []GitHubActivity `json:"issues"`
	Commits      []GitHubActivity `json:"commits"`
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
		SELECT repository, date, url, count, activity_type, COALESCE(github_id, '') as github_id
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
		var repo, dateStr, url, activityType, githubID string
		var count int
		err := rows.Scan(&repo, &dateStr, &url, &count, &activityType, &githubID)
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
			GitHubID:     githubID,
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

	// Enable connection pooling
	app.DB.SetMaxOpenConns(25)
	app.DB.SetMaxIdleConns(5)
	app.DB.SetConnMaxLifetime(5 * time.Minute)

	// Create tables without the unique index initially
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

	CREATE TABLE IF NOT EXISTS pr_comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		repository TEXT NOT NULL,
		pr_number INTEGER NOT NULL,
		pr_title TEXT NOT NULL,
		author TEXT NOT NULL,
		body TEXT,
		created_at TEXT NOT NULL,
		pr_url TEXT,
		comment_url TEXT,
		fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_pr_comments_repo ON pr_comments(repository);
	CREATE INDEX IF NOT EXISTS idx_pr_comments_created ON pr_comments(created_at);
	`

	_, err = app.DB.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// Migration: Add github_id column if it doesn't exist
	var columnExists bool
	err = app.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('github_activity') 
		WHERE name = 'github_id'
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("failed to check for github_id column: %w", err)
	}

	if !columnExists {
		// Add the github_id column to existing table
		_, err = app.DB.Exec(`ALTER TABLE github_activity ADD COLUMN github_id TEXT NOT NULL DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("failed to add github_id column: %w", err)
		}
	}

	// Create the unique index (this will work whether the column was just added or already exists)
	_, err = app.DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_activity ON github_activity(date, repository, activity_type, github_id)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index: %w", err)
	}

	return nil
}

func (app *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func (app *App) getActivityHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := app.DB.Query(`
		SELECT id, date, repository, activity_type, count, COALESCE(url, '') as url, COALESCE(github_id, '') as github_id
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
		err := rows.Scan(&activity.ID, &dateStr, &activity.Repository, &activity.ActivityType, &activity.Count, &activity.URL, &activity.GitHubID)
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

	// Insert new activity data, ignoring duplicates based on unique constraint
	for _, activity := range activities {
		_, err := app.DB.Exec(`
			INSERT OR IGNORE INTO github_activity (date, repository, activity_type, count, url, github_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`, activity.Date.Format("2006-01-02"), activity.Repository, activity.ActivityType, activity.Count, activity.URL, activity.GitHubID)
		if err != nil {
			return fmt.Errorf("failed to insert activity: %w", err)
		}
	}

	// Fetch PR comments for repositories with recent activity
	prComments, err := app.GitHubService.FetchPRComments(username)
	if err != nil {
		// Log error but don't fail the whole refresh
		fmt.Printf("Warning: Failed to fetch PR comments: %v\n", err)
	} else {
		// Clear old PR comments
		_, err = app.DB.Exec("DELETE FROM pr_comments WHERE created_at < date('now', '-180 days')")
		if err != nil {
			fmt.Printf("Warning: Failed to clear old PR comments: %v\n", err)
		}

		// Insert new PR comments
		for _, comment := range prComments {
			_, err := app.DB.Exec(`
				INSERT OR REPLACE INTO pr_comments 
				(repository, pr_number, pr_title, author, body, created_at, pr_url, comment_url)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, comment.Repository, comment.PRNumber, comment.PRTitle, comment.Author, 
				comment.Body, comment.CreatedAt.Format(time.RFC3339), comment.PRURL, comment.CommentURL)
			if err != nil {
				fmt.Printf("Warning: Failed to insert PR comment: %v\n", err)
			}
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

// Handler for /api/projects: returns blog-style listing of projects with recent PR comments
func (app *App) getProjectsHandler(w http.ResponseWriter, r *http.Request) {
	sixMonthsAgo := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	
	// Get all repositories with activity
	rows, err := app.DB.Query(`
		SELECT repository, MAX(date) as latest_date, 
		       SUM(CASE WHEN activity_type = 'commit' THEN count ELSE 0 END) as total_commits,
		       GROUP_CONCAT(DISTINCT activity_type) as activity_types,
		       url
		FROM github_activity
		WHERE date >= ?
		GROUP BY repository
		ORDER BY latest_date DESC
	`, sixMonthsAgo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []ProjectEntry
	for rows.Next() {
		var repo, latestDateStr, activityTypesStr, url string
		var totalCommits int
		err := rows.Scan(&repo, &latestDateStr, &totalCommits, &activityTypesStr, &url)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		latestDate, _ := time.Parse("2006-01-02", latestDateStr)
		
		// Split activity types
		var activityTypes []string
		if activityTypesStr != "" {
			activityTypes = strings.Split(activityTypesStr, ",")
		}

		// Get recent PR comments for this repo (last 5)
		comments, err := app.getPRCommentsForRepo(repo, 5)
		if err != nil {
			// Log error but continue
			fmt.Printf("Warning: Failed to fetch PR comments for %s: %v\n", repo, err)
			comments = []PRComment{}
		}

		projects = append(projects, ProjectEntry{
			Repository:     repo,
			LatestDate:     latestDate,
			TotalCommits:   totalCommits,
			ActivityTypes:  activityTypes,
			RecentComments: comments,
			URL:            url,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

// Handler for /api/blog: returns blog-style listing grouped by repository with all activity types
func (app *App) getBlogHandler(w http.ResponseWriter, r *http.Request) {
	sixMonthsAgo := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	
	// Get all activities from the database
	rows, err := app.DB.Query(`
		SELECT repository, date, activity_type, count, COALESCE(url, '') as url, COALESCE(github_id, '') as github_id
		FROM github_activity
		WHERE date >= ?
		ORDER BY date DESC
	`, sixMonthsAgo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Group activities by repository
	repoActivities := make(map[string]struct {
		latestDate   time.Time
		url          string
		pullRequests []GitHubActivity
		issues       []GitHubActivity
		commits      []GitHubActivity
	})

	for rows.Next() {
		var repo, dateStr, activityType, url, githubID string
		var count int
		err := rows.Scan(&repo, &dateStr, &activityType, &count, &url, &githubID)
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
			GitHubID:     githubID,
		}

		repoData := repoActivities[repo]
		if repoData.latestDate.IsZero() || date.After(repoData.latestDate) {
			repoData.latestDate = date
		}
		if repoData.url == "" {
			repoData.url = url
		}

		// Group by activity type
		switch activityType {
		case "pull_request":
			repoData.pullRequests = append(repoData.pullRequests, activity)
		case "issue":
			repoData.issues = append(repoData.issues, activity)
		case "commit", "review", "repository", "fork", "star", "activity":
			// All commit-like activities go into commits section
			repoData.commits = append(repoData.commits, activity)
		default:
			// Log unknown activity types for debugging
			fmt.Printf("Unknown activity type '%s' found for repo %s, adding to commits\n", activityType, repo)
			repoData.commits = append(repoData.commits, activity)
		}

		repoActivities[repo] = repoData
	}

	// Convert to BlogEntry slice
	var blogEntries []BlogEntry
	for repo, data := range repoActivities {
		blogEntries = append(blogEntries, BlogEntry{
			Repository:   repo,
			LatestDate:   data.latestDate,
			URL:          data.url,
			PullRequests: data.pullRequests,
			Issues:       data.issues,
			Commits:      data.commits,
		})
	}

	// Sort by most recent activity
	sort.Slice(blogEntries, func(i, j int) bool {
		return blogEntries[i].LatestDate.After(blogEntries[j].LatestDate)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blogEntries)
}

func (app *App) getPRCommentsForRepo(repo string, limit int) ([]PRComment, error) {
	rows, err := app.DB.Query(`
		SELECT id, repository, pr_number, pr_title, author, body, created_at, pr_url, comment_url
		FROM pr_comments
		WHERE repository = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, repo, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []PRComment
	for rows.Next() {
		var comment PRComment
		var createdAtStr string
		err := rows.Scan(&comment.ID, &comment.Repository, &comment.PRNumber, &comment.PRTitle, 
			&comment.Author, &comment.Body, &createdAtStr, &comment.PRURL, &comment.CommentURL)
		if err != nil {
			return nil, err
		}
		parsedTime, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			// Log error and use current time as fallback
			fmt.Printf("Warning: Failed to parse comment timestamp %s: %v\n", createdAtStr, err)
			comment.CreatedAt = time.Now()
		} else {
			comment.CreatedAt = parsedTime
		}
		comments = append(comments, comment)
	}

	return comments, nil
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
	r.HandleFunc("/api/projects", app.getProjectsHandler)
	r.HandleFunc("/api/blog", app.getBlogHandler)
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
