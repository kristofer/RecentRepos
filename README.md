# RecentRepos

A web application that displays a timeline of your GitHub activity, showing repositories you've worked on over the past few weeks. It provides a backward-looking review of your contribution activity including commits, pull requests, issues, and reviews.

![RecentRepos Timeline](https://github.com/user-attachments/assets/2f43f9d9-7904-413c-9186-146ed81426bc)

## Features

- **Timeline View**: Shows your GitHub activity in a reverse chronological timeline
- **Activity Types**: Displays commits, pull requests, issues, reviews, and other repository activities
- **Repository Links**: Click on repository names to navigate to the GitHub repository
- **Real-time Refresh**: Fetch the latest activity data with the refresh button
- **GitHub-like UI**: Dark theme interface resembling GitHub's contribution graph
- **3-Tier Architecture**: Go backend, SQLite database, and HTML/CSS/JS frontend

## Setup

### Prerequisites

- Go 1.21 or later
- Git

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/kristofer/RecentRepos.git
   cd RecentRepos
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o recentrepos .
   ```

### Configuration

#### Environment Variables

- `GITHUB_TOKEN` (optional): Your GitHub personal access token for API access
- `GITHUB_USERNAME` (optional): Your GitHub username (defaults to "kristofer")
- `PORT` (optional): Port to run the server on (defaults to 8080)

#### GitHub Token Setup

For real GitHub data, create a personal access token:

1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate a new token with `repo` and `user` scopes
3. Set the environment variable:
   ```bash
   export GITHUB_TOKEN=your_token_here
   export GITHUB_USERNAME=your_username
   ```

### Running

```bash
./recentrepos
```

Then open your browser to `http://localhost:8080`

## Architecture

### 3-Tier Web Architecture

1. **Presentation Tier**: HTML/CSS/JavaScript frontend served by the Go web server
2. **Logic Tier**: Go web server with REST API endpoints and GitHub API integration
3. **Data Tier**: SQLite3 database for storing and caching activity data

### API Endpoints

- `GET /` - Main application page
- `GET /api/activity` - Fetch stored activity data
- `POST /api/refresh` - Refresh activity data from GitHub API
- `GET /static/*` - Static assets (CSS, JS)

## Usage

1. Start the application
2. Open `http://localhost:8080` in your browser
3. Click "Refresh Activity" to fetch your GitHub activity
4. Browse your timeline of repository activity

The application will show sample data if no GitHub token is configured, or fetch real data from the GitHub API if properly configured.

## Development

### Project Structure

```
.
├── main.go          # Main application and web server
├── github.go        # GitHub API integration service
├── go.mod           # Go module dependencies
├── static/          # Frontend assets
│   ├── index.html   # Main application page
│   ├── style.css    # Styling with GitHub-like theme
│   └── script.js    # Frontend JavaScript application
└── activity.db     # SQLite database (created automatically)
```

### Database Schema

The application uses SQLite with a simple schema:

```sql
CREATE TABLE github_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    repository TEXT NOT NULL,
    activity_type TEXT NOT NULL,
    count INTEGER DEFAULT 1,
    url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## License

MIT License - see LICENSE file for details
