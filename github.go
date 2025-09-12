package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type GitHubService struct {
	Token string
}

type GitHubEvent struct {
	Type      string      `json:"type"`
	Repo      GitHubRepo  `json:"repo"`
	CreatedAt time.Time   `json:"created_at"`
	Payload   interface{} `json:"payload"`
}

type GitHubRepo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	URL      string `json:"url"`
	HTMLURL  string `json:"html_url"`
}

type GitHubCommit struct {
	SHA    string           `json:"sha"`
	Commit GitHubCommitData `json:"commit"`
	URL    string           `json:"html_url"`
}

type GitHubCommitData struct {
	Message string                `json:"message"`
	Author  GitHubCommitAuthor    `json:"author"`
}

type GitHubCommitAuthor struct {
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

func NewGitHubService() *GitHubService {
	token := os.Getenv("GITHUB_TOKEN")
	return &GitHubService{Token: token}
}

func (g *GitHubService) FetchUserActivity(username string) ([]GitHubActivity, error) {
	if g.Token == "" {
		// Return sample data if no token is provided
		return g.getSampleData(), nil
	}

	// First fetch user repos
	repos, err := g.fetchUserRepos(username)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user repos: %w", err)
	}

	// Then fetch commits for each repo
	var allActivities []GitHubActivity
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)

	for _, repo := range repos {
		commits, err := g.fetchRepoCommits(username, repo.Name, sixMonthsAgo)
		if err != nil {
			// Log error but continue with other repos
			fmt.Printf("Warning: Failed to fetch commits for %s: %v\n", repo.Name, err)
			continue
		}
		allActivities = append(allActivities, commits...)
	}

	// Also fetch recent events for other activity types
	events, err := g.fetchRecentEvents(username)
	if err == nil {
		allActivities = append(allActivities, g.convertEventsToActivity(events)...)
	}

	return allActivities, nil
}

func (g *GitHubService) convertEventsToActivity(events []GitHubEvent) []GitHubActivity {
	activityMap := make(map[string]*GitHubActivity)

	for _, event := range events {
		date := event.CreatedAt.Format("2006-01-02")
		key := fmt.Sprintf("%s-%s-%s", date, event.Repo.Name, g.getActivityType(event.Type))

		if activity, exists := activityMap[key]; exists {
			activity.Count++
		} else {
			activityMap[key] = &GitHubActivity{
				Date:         event.CreatedAt,
				Repository:   event.Repo.Name,
				ActivityType: g.getActivityType(event.Type),
				Count:        1,
				URL:          fmt.Sprintf("https://github.com/%s", event.Repo.Name),
			}
		}
	}

	var activities []GitHubActivity
	for _, activity := range activityMap {
		activities = append(activities, *activity)
	}

	return activities
}

func (g *GitHubService) fetchUserRepos(username string) ([]GitHubRepo, error) {
	var allRepos []GitHubRepo
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("https://api.github.com/users/%s/repos?type=all&sort=pushed&per_page=%d&page=%d", username, perPage, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "token "+g.Token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
		}

		var repos []GitHubRepo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			return nil, err
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)
		
		// Check if we got less than perPage, meaning we've reached the end
		if len(repos) < perPage {
			break
		}
		
		page++
	}

	return allRepos, nil
}

func (g *GitHubService) fetchRepoCommits(username, repoName string, since time.Time) ([]GitHubActivity, error) {
	var allCommits []GitHubCommit
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?author=%s&since=%s&per_page=%d&page=%d", 
			username, repoName, username, since.Format(time.RFC3339), perPage, page)
		
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "token "+g.Token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 409 {
			// Repository is empty, skip it
			return []GitHubActivity{}, nil
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub API returned status: %d for repo %s", resp.StatusCode, repoName)
		}

		var commits []GitHubCommit
		if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
			return nil, err
		}

		if len(commits) == 0 {
			break
		}

		allCommits = append(allCommits, commits...)
		
		// Check if we got less than perPage, meaning we've reached the end
		if len(commits) < perPage {
			break
		}
		
		page++
	}

	return g.convertCommitsToActivity(allCommits, repoName, username), nil
}

func (g *GitHubService) fetchRecentEvents(username string) ([]GitHubEvent, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/events", username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var events []GitHubEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, err
	}

	// Filter events to last 6 months
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	var recentEvents []GitHubEvent
	for _, event := range events {
		if event.CreatedAt.After(sixMonthsAgo) {
			recentEvents = append(recentEvents, event)
		}
	}
	return recentEvents, nil
}

func (g *GitHubService) convertCommitsToActivity(commits []GitHubCommit, repoName, username string) []GitHubActivity {
	// Group commits by date
	commitsByDate := make(map[string]int)
	commitDates := make(map[string]time.Time)
	
	for _, commit := range commits {
		dateStr := commit.Commit.Author.Date.Format("2006-01-02")
		commitsByDate[dateStr]++
		if _, exists := commitDates[dateStr]; !exists {
			commitDates[dateStr] = commit.Commit.Author.Date
		}
	}

	var activities []GitHubActivity
	for dateStr, count := range commitsByDate {
		activities = append(activities, GitHubActivity{
			Date:         commitDates[dateStr],
			Repository:   fmt.Sprintf("%s/%s", username, repoName),
			ActivityType: "commit",
			Count:        count,
			URL:          fmt.Sprintf("https://github.com/%s/%s", username, repoName),
		})
	}

	return activities
}

func (g *GitHubService) getActivityType(eventType string) string {
	switch eventType {
	case "PushEvent":
		return "commit"
	case "PullRequestEvent":
		return "pull_request"
	case "IssuesEvent":
		return "issue"
	case "PullRequestReviewEvent":
		return "review"
	case "CreateEvent", "DeleteEvent":
		return "repository"
	case "ForkEvent":
		return "fork"
	case "WatchEvent":
		return "star"
	default:
		return "activity"
	}
}

func (g *GitHubService) getSampleData() []GitHubActivity {
	now := time.Now()
	return []GitHubActivity{
		{
			Date:         now.AddDate(0, 0, -1),
			Repository:   "kristofer/RecentRepos",
			ActivityType: "commit",
			Count:        3,
			URL:          "https://github.com/kristofer/RecentRepos",
		},
		{
			Date:         now.AddDate(0, 0, -2),
			Repository:   "kristofer/example-project",
			ActivityType: "pull_request",
			Count:        1,
			URL:          "https://github.com/kristofer/example-project",
		},
		{
			Date:         now.AddDate(0, 0, -3),
			Repository:   "kristofer/another-repo",
			ActivityType: "commit",
			Count:        5,
			URL:          "https://github.com/kristofer/another-repo",
		},
		{
			Date:         now.AddDate(0, 0, -5),
			Repository:   "kristofer/web-app",
			ActivityType: "issue",
			Count:        2,
			URL:          "https://github.com/kristofer/web-app",
		},
		{
			Date:         now.AddDate(0, 0, -7),
			Repository:   "kristofer/RecentRepos",
			ActivityType: "review",
			Count:        1,
			URL:          "https://github.com/kristofer/RecentRepos",
		},
		{
			Date:         now.AddDate(0, 0, -10),
			Repository:   "kristofer/mobile-app",
			ActivityType: "commit",
			Count:        8,
			URL:          "https://github.com/kristofer/mobile-app",
		},
	}
}
