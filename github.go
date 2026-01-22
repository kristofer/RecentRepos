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
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Repo      GitHubRepo             `json:"repo"`
	CreatedAt time.Time              `json:"created_at"`
	Payload   map[string]interface{} `json:"payload"`
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

type GitHubPullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	User      GitHubUser `json:"user"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GitHubIssueComment struct {
	ID        int        `json:"id"`
	User      GitHubUser `json:"user"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	HTMLURL   string     `json:"html_url"`
}

type GitHubUser struct {
	Login string `json:"login"`
}

type GitHubPRReviewComment struct {
	ID        int        `json:"id"`
	User      GitHubUser `json:"user"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	HTMLURL   string     `json:"html_url"`
	PullRequestURL string `json:"pull_request_url"`
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
	var activities []GitHubActivity

	for _, event := range events {
		activityType := g.getActivityType(event.Type)
		githubID := event.ID
		url := fmt.Sprintf("https://github.com/%s", event.Repo.Name)
		
		// Extract specific IDs and URLs from payload based on event type
		switch event.Type {
		case "PullRequestEvent":
			if pr, ok := event.Payload["pull_request"].(map[string]interface{}); ok {
				if number, ok := pr["number"].(float64); ok {
					githubID = fmt.Sprintf("pr-%d", int(number))
					if htmlURL, ok := pr["html_url"].(string); ok {
						url = htmlURL
					}
				}
			}
		case "IssuesEvent":
			if issue, ok := event.Payload["issue"].(map[string]interface{}); ok {
				if number, ok := issue["number"].(float64); ok {
					githubID = fmt.Sprintf("issue-%d", int(number))
					if htmlURL, ok := issue["html_url"].(string); ok {
						url = htmlURL
					}
				}
			}
		case "PushEvent":
			// Skip push events as we already track individual commits separately
			// via the fetchRepoCommits function which provides more detailed information
			continue
		}

		activities = append(activities, GitHubActivity{
			Date:         event.CreatedAt,
			Repository:   event.Repo.Name,
			ActivityType: activityType,
			Count:        1,
			URL:          url,
			GitHubID:     githubID,
		})
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
	// Store each commit individually with its unique SHA
	var activities []GitHubActivity
	
	for _, commit := range commits {
		activities = append(activities, GitHubActivity{
			Date:         commit.Commit.Author.Date,
			Repository:   fmt.Sprintf("%s/%s", username, repoName),
			ActivityType: "commit",
			Count:        1,
			URL:          commit.URL,
			GitHubID:     commit.SHA,
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
			Count:        1,
			URL:          "https://github.com/kristofer/RecentRepos/commit/abc123",
			GitHubID:     "abc123",
		},
		{
			Date:         now.AddDate(0, 0, -1),
			Repository:   "kristofer/RecentRepos",
			ActivityType: "commit",
			Count:        1,
			URL:          "https://github.com/kristofer/RecentRepos/commit/def456",
			GitHubID:     "def456",
		},
		{
			Date:         now.AddDate(0, 0, -1),
			Repository:   "kristofer/RecentRepos",
			ActivityType: "commit",
			Count:        1,
			URL:          "https://github.com/kristofer/RecentRepos/commit/ghi789",
			GitHubID:     "ghi789",
		},
		{
			Date:         now.AddDate(0, 0, -2),
			Repository:   "kristofer/example-project",
			ActivityType: "pull_request",
			Count:        1,
			URL:          "https://github.com/kristofer/example-project/pull/42",
			GitHubID:     "pr-42",
		},
		{
			Date:         now.AddDate(0, 0, -3),
			Repository:   "kristofer/another-repo",
			ActivityType: "commit",
			Count:        1,
			URL:          "https://github.com/kristofer/another-repo/commit/jkl012",
			GitHubID:     "jkl012",
		},
		{
			Date:         now.AddDate(0, 0, -3),
			Repository:   "kristofer/another-repo",
			ActivityType: "commit",
			Count:        1,
			URL:          "https://github.com/kristofer/another-repo/commit/mno345",
			GitHubID:     "mno345",
		},
		{
			Date:         now.AddDate(0, 0, -5),
			Repository:   "kristofer/web-app",
			ActivityType: "issue",
			Count:        1,
			URL:          "https://github.com/kristofer/web-app/issues/15",
			GitHubID:     "issue-15",
		},
		{
			Date:         now.AddDate(0, 0, -5),
			Repository:   "kristofer/web-app",
			ActivityType: "issue",
			Count:        1,
			URL:          "https://github.com/kristofer/web-app/issues/16",
			GitHubID:     "issue-16",
		},
		{
			Date:         now.AddDate(0, 0, -7),
			Repository:   "kristofer/RecentRepos",
			ActivityType: "review",
			Count:        1,
			URL:          "https://github.com/kristofer/RecentRepos",
			GitHubID:     "review-1",
		},
		{
			Date:         now.AddDate(0, 0, -10),
			Repository:   "kristofer/mobile-app",
			ActivityType: "commit",
			Count:        1,
			URL:          "https://github.com/kristofer/mobile-app/commit/pqr678",
			GitHubID:     "pqr678",
		},
	}
}

// FetchPRComments fetches PR comments from all repositories for a user
func (g *GitHubService) FetchPRComments(username string) ([]PRComment, error) {
	if g.Token == "" {
		// Return sample PR comments if no token is provided
		return g.getSamplePRComments(), nil
	}

	var allComments []PRComment

	// First fetch user repos
	repos, err := g.fetchUserRepos(username)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user repos: %w", err)
	}

	sixMonthsAgo := time.Now().AddDate(0, -6, 0)

	// For each repo, fetch recent PRs and their comments
	for _, repo := range repos {
		// Extract repo name from full name (username/repo)
		repoName := repo.Name
		
		// Fetch PRs for this repo
		prs, err := g.fetchRepoPullRequests(username, repoName)
		if err != nil {
			fmt.Printf("Warning: Failed to fetch PRs for %s: %v\n", repoName, err)
			continue
		}

		// For each PR, fetch comments (limit to first 5 PRs)
		prLimit := len(prs)
		if prLimit > 5 {
			prLimit = 5
		}
		for i := 0; i < prLimit; i++ {
			pr := prs[i]
			if pr.UpdatedAt.Before(sixMonthsAgo) {
				continue
			}

			// Fetch issue comments (PR comments on the conversation)
			issueComments, err := g.fetchPRIssueComments(username, repoName, pr.Number)
			if err != nil {
				fmt.Printf("Warning: Failed to fetch issue comments for PR #%d: %v\n", pr.Number, err)
			} else {
				for _, comment := range issueComments {
					if comment.CreatedAt.After(sixMonthsAgo) {
						allComments = append(allComments, PRComment{
							Repository: fmt.Sprintf("%s/%s", username, repoName),
							PRNumber:   pr.Number,
							PRTitle:    pr.Title,
							Author:     comment.User.Login,
							Body:       comment.Body,
							CreatedAt:  comment.CreatedAt,
							PRURL:      pr.HTMLURL,
							CommentURL: comment.HTMLURL,
						})
					}
				}
			}
		}
	}

	return allComments, nil
}

func (g *GitHubService) fetchRepoPullRequests(username, repoName string) ([]GitHubPullRequest, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=all&sort=updated&direction=desc&per_page=10", username, repoName)
	
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
		return []GitHubPullRequest{}, nil // Return empty if no PRs or access denied
	}

	var prs []GitHubPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}

	return prs, nil
}

func (g *GitHubService) fetchPRIssueComments(username, repoName string, prNumber int) ([]GitHubIssueComment, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments?per_page=5", username, repoName, prNumber)
	
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
		return []GitHubIssueComment{}, nil
	}

	var comments []GitHubIssueComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, err
	}

	return comments, nil
}

func (g *GitHubService) getSamplePRComments() []PRComment {
	now := time.Now()
	return []PRComment{
		{
			Repository: "kristofer/RecentRepos",
			PRNumber:   1,
			PRTitle:    "Add initial timeline feature",
			Author:     "kristofer",
			Body:       "This looks great! The timeline view is very clean and easy to read.",
			CreatedAt:  now.AddDate(0, 0, -1),
			PRURL:      "https://github.com/kristofer/RecentRepos/pull/1",
			CommentURL: "https://github.com/kristofer/RecentRepos/pull/1#issuecomment-1",
		},
		{
			Repository: "kristofer/RecentRepos",
			PRNumber:   2,
			PRTitle:    "Improve database schema",
			Author:     "copilot",
			Body:       "Good optimization! The indexes will help with query performance.",
			CreatedAt:  now.AddDate(0, 0, -2),
			PRURL:      "https://github.com/kristofer/RecentRepos/pull/2",
			CommentURL: "https://github.com/kristofer/RecentRepos/pull/2#issuecomment-2",
		},
		{
			Repository: "kristofer/example-project",
			PRNumber:   15,
			PRTitle:    "Fix authentication bug",
			Author:     "kristofer",
			Body:       "LGTM! This fixes the issue we were seeing in production.",
			CreatedAt:  now.AddDate(0, 0, -3),
			PRURL:      "https://github.com/kristofer/example-project/pull/15",
			CommentURL: "https://github.com/kristofer/example-project/pull/15#issuecomment-15",
		},
		{
			Repository: "kristofer/another-repo",
			PRNumber:   8,
			PRTitle:    "Update dependencies",
			Author:     "dependabot",
			Body:       "Bumps version from 1.2.3 to 1.2.4. See release notes for details.",
			CreatedAt:  now.AddDate(0, 0, -5),
			PRURL:      "https://github.com/kristofer/another-repo/pull/8",
			CommentURL: "https://github.com/kristofer/another-repo/pull/8#issuecomment-8",
		},
	}
}
