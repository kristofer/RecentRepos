# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

RecentRepos is a Go-based web application that displays a timeline of GitHub activity. It uses a 3-tier architecture:
- **Frontend**: Static HTML/CSS/JavaScript served from `/static/`
- **Backend**: Go HTTP server with REST API endpoints
- **Database**: SQLite3 database (`activity.db`) for caching GitHub activity data

## Build and Run Commands

```bash
# Install dependencies
go mod tidy

# Build the application
go build -o recentrepos .

# Run the application
./recentrepos
```

The server runs on port 8080 by default (configurable via `PORT` environment variable).

## Environment Configuration

- `GITHUB_TOKEN`: GitHub personal access token for API access (optional - uses sample data if not provided)
- `GITHUB_USERNAME`: GitHub username (defaults to "kristofer")
- `PORT`: Server port (defaults to 8080)

## Architecture Details

### Core Components

- `main.go`: Main application with HTTP server, database operations, and API handlers
- `github.go`: GitHub API integration service with sample data fallback
- `static/`: Frontend assets (HTML/CSS/JavaScript)

### Database Schema

SQLite table `github_activity` with fields: id, date, repository, activity_type, count, url, created_at
Indexed on date and repository for query performance.

### API Endpoints

- `GET /`: Main application page
- `GET /api/activity`: Recent activity timeline (last 100 items)
- `GET /api/commits`: 6-month commit history grouped by repository
- `POST /api/refresh`: Refresh activity data from GitHub API
- `GET /api/status`: Application status and configuration
- `GET /static/*`: Static file serving

### Key Data Structures

- `App`: Main application struct containing database connection and GitHub service
- `GitHubActivity`: Activity record with date, repository, type, count, and URL
- `GitHubService`: GitHub API integration with token-based authentication

### Activity Types

The application tracks: commits, pull_requests, issues, reviews, repository events, forks, and stars.

## Development Notes

- Uses standard library HTTP server (no external web framework like Gin)
- SQLite database is created automatically on first run
- Sample data mode available when no GitHub token is configured
- Frontend uses vanilla JavaScript with tab-based navigation
- All database queries use parameterized statements for security