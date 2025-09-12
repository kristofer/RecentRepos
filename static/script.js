class RecentRepos {
    constructor() {
        this.activities = [];
        this.init();
    }

    init() {
        this.loadStatus();
        this.loadActivity();
        this.setupEventListeners();
    }

    setupEventListeners() {
        const refreshBtn = document.getElementById('refresh-btn');
        refreshBtn.addEventListener('click', () => this.refreshActivity());
    }

    async loadStatus() {
        try {
            const response = await fetch('/api/status');
            if (response.ok) {
                const status = await response.json();
                this.showStatus(status);
            }
        } catch (error) {
            console.error('Failed to load status:', error);
        }
    }

    showStatus(status) {
        const statusIndicator = document.getElementById('status-indicator');
        if (status.sample_mode) {
            statusIndicator.textContent = 'Sample Mode - Configure GITHUB_TOKEN for real data';
            statusIndicator.className = 'status-indicator sample-mode';
        } else {
            statusIndicator.textContent = `GitHub Mode - ${status.github_username}`;
            statusIndicator.className = 'status-indicator github-mode';
        }
    }

    async loadActivity() {
        try {
            this.showLoading(true);
            const response = await fetch('/api/activity');
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            this.activities = await response.json();
            this.renderActivity();
        } catch (error) {
            this.showError('Failed to load activity: ' + error.message);
        } finally {
            this.showLoading(false);
        }
    }

    async refreshActivity() {
        const refreshBtn = document.getElementById('refresh-btn');
        refreshBtn.disabled = true;
        refreshBtn.textContent = 'Refreshing...';

        try {
            const response = await fetch('/api/refresh', { method: 'POST' });
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            await this.loadActivity();
        } catch (error) {
            this.showError('Failed to refresh activity: ' + error.message);
        } finally {
            refreshBtn.disabled = false;
            refreshBtn.textContent = 'Refresh Activity';
        }
    }

    renderActivity() {
        const timeline = document.getElementById('activity-timeline');
        
        if (this.activities.length === 0) {
            timeline.innerHTML = `
                <div class="activity-item">
                    <p>No activity data available. Click "Refresh Activity" to fetch your GitHub activity.</p>
                </div>
            `;
            return;
        }

        timeline.innerHTML = this.activities.map(activity => `
            <div class="activity-item">
                <div class="activity-header">
                    <span class="activity-date">${this.formatDate(activity.date)}</span>
                    <span class="activity-type ${activity.activity_type}">${activity.activity_type}</span>
                </div>
                <a href="${activity.url}" class="repository-name" target="_blank">
                    ${activity.repository}
                </a>
                <div class="activity-count">
                    ${activity.count} ${activity.activity_type}${activity.count > 1 ? 's' : ''}
                </div>
            </div>
        `).join('');
    }

    formatDate(dateString) {
        const date = new Date(dateString);
        const now = new Date();
        const diffTime = Math.abs(now - date);
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

        if (diffDays === 1) {
            return 'Yesterday';
        } else if (diffDays < 7) {
            return `${diffDays} days ago`;
        } else if (diffDays < 30) {
            const weeks = Math.floor(diffDays / 7);
            return `${weeks} week${weeks > 1 ? 's' : ''} ago`;
        } else {
            return date.toLocaleDateString('en-US', { 
                year: 'numeric', 
                month: 'short', 
                day: 'numeric' 
            });
        }
    }

    showLoading(show) {
        const loading = document.getElementById('loading');
        const timeline = document.getElementById('activity-timeline');
        
        if (show) {
            loading.style.display = 'block';
            timeline.style.display = 'none';
        } else {
            loading.style.display = 'none';
            timeline.style.display = 'block';
        }
    }

    showError(message) {
        const errorDiv = document.getElementById('error');
        errorDiv.textContent = message;
        errorDiv.style.display = 'block';
        
        setTimeout(() => {
            errorDiv.style.display = 'none';
        }, 5000);
    }
}

// Initialize the app when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new RecentRepos();
});