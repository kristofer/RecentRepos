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
	Type      string    `json:"type"`
	Repo      GitHubRepo `json:"repo"`
	CreatedAt time.Time `json:"created_at"`
	Payload   interface{} `json:"payload"`
}

type GitHubRepo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GitHubCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	URL     string `json:"html_url"`
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

	return g.convertEventsToActivity(events), nil
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