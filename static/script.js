class RecentRepos {
    constructor() {
        this.activities = [];
        this.commitsData = [];
        this.currentCommitsPage = 1;
        this.commitsPerPage = 100;
        this.commitsPagination = null;
        this.init();
    }

    init() {
        this.loadStatus();
        this.loadActivity();
        this.setupEventListeners();
        this.setupTabs();
    }

    setupEventListeners() {
        const refreshBtn = document.getElementById('refresh-btn');
        refreshBtn.addEventListener('click', () => this.refreshActivity());
    }

    setupTabs() {
        const tabActivity = document.getElementById('tab-activity');
        const tabCommits = document.getElementById('tab-commits');
        const activityTimeline = document.getElementById('activity-timeline');
        const commitsTimeline = document.getElementById('commits-timeline');

        tabActivity.addEventListener('click', () => {
            tabActivity.classList.add('active');
            tabCommits.classList.remove('active');
            activityTimeline.style.display = '';
            activityTimeline.classList.add('active');
            commitsTimeline.style.display = 'none';
            commitsTimeline.classList.remove('active');
        });

        tabCommits.addEventListener('click', () => {
            tabCommits.classList.add('active');
            tabActivity.classList.remove('active');
            commitsTimeline.style.display = '';
            commitsTimeline.classList.add('active');
            activityTimeline.style.display = 'none';
            activityTimeline.classList.remove('active');
            this.loadCommits(1);
        });
    }
    async loadCommits(page = 1) {
        const commitsTimeline = document.getElementById('commits-timeline');
        if (page === 1) {
            commitsTimeline.innerHTML = '<div class="loading">Loading commits...</div>';
        }
        try {
            const response = await fetch(`/api/commits?page=${page}&limit=${this.commitsPerPage}`);
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const result = await response.json();
            this.commitsData = result.data || [];
            this.commitsPagination = result.pagination || null;
            this.currentCommitsPage = page;
            this.renderCommits();
        } catch (error) {
            commitsTimeline.innerHTML = `<div class="error">Failed to load commits: ${error.message}</div>`;
        }
    }

    renderCommits() {
        const commitsTimeline = document.getElementById('commits-timeline');
        if (!this.commitsData || this.commitsData.length === 0) {
            commitsTimeline.innerHTML = '<div class="activity-item">No commit data available.</div>';
            return;
        }

        let content = this.commitsData.map(repoGroup => `
            <div class="activity-item">
                <a href="${repoGroup.commits[0].url}" class="repository-name" target="_blank">${repoGroup.repository}</a>
                <div class="commits-list">
                    ${repoGroup.commits.map(commit => `
                        <div class="commit-entry">
                            <span class="activity-date">${this.formatDate(commit.date)}</span>
                            <span>${commit.count} commit${commit.count > 1 ? 's' : ''}</span>
                        </div>
                    `).join('')}
                </div>
            </div>
        `).join('');

        // Add pagination controls if needed
        if (this.commitsPagination && this.commitsPagination.total_pages > 1) {
            content += this.renderPaginationControls();
        }

        commitsTimeline.innerHTML = content;
        this.setupPaginationEventListeners();
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

    renderPaginationControls() {
        const p = this.commitsPagination;
        let paginationHtml = '<div class="pagination">';
        
        // Previous button
        if (p.has_prev) {
            paginationHtml += `<button class="page-btn" data-page="${p.page - 1}">← Previous</button>`;
        } else {
            paginationHtml += `<button class="page-btn disabled">← Previous</button>`;
        }
        
        // Page info
        paginationHtml += `<span class="page-info">Page ${p.page} of ${p.total_pages} (${p.total} repositories)</span>`;
        
        // Next button
        if (p.has_next) {
            paginationHtml += `<button class="page-btn" data-page="${p.page + 1}">Next →</button>`;
        } else {
            paginationHtml += `<button class="page-btn disabled">Next →</button>`;
        }
        
        paginationHtml += '</div>';
        return paginationHtml;
    }

    setupPaginationEventListeners() {
        const pageButtons = document.querySelectorAll('.page-btn:not(.disabled)');
        pageButtons.forEach(button => {
            button.addEventListener('click', (e) => {
                const page = parseInt(e.target.getAttribute('data-page'));
                if (page && page !== this.currentCommitsPage) {
                    this.loadCommits(page);
                }
            });
        });
    }
}

// Initialize the app when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new RecentRepos();
});