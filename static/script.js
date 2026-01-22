class RecentRepos {
    constructor() {
        this.activities = [];
        this.commitsData = [];
        this.projectsData = [];
        this.blogData = [];
        this.currentCommitsPage = 1;
        this.commitsPerPage = 100;
        this.commitsPagination = null;
        this.init();
    }

    init() {
        this.loadStatus();
        this.loadBlog();
        this.setupEventListeners();
        this.setupTabs();
    }

    setupEventListeners() {
        const refreshBtn = document.getElementById('refresh-btn');
        refreshBtn.addEventListener('click', () => this.refreshActivity());
    }

    setupTabs() {
        const tabBlog = document.getElementById('tab-blog');
        const tabActivity = document.getElementById('tab-activity');
        const tabCommits = document.getElementById('tab-commits');
        const tabProjects = document.getElementById('tab-projects');
        const blogTimeline = document.getElementById('blog-timeline');
        const activityTimeline = document.getElementById('activity-timeline');
        const commitsTimeline = document.getElementById('commits-timeline');
        const projectsTimeline = document.getElementById('projects-timeline');

        tabBlog.addEventListener('click', () => {
            this.activateTab(tabBlog, [tabActivity, tabCommits, tabProjects]);
            this.showContent(blogTimeline, [activityTimeline, commitsTimeline, projectsTimeline]);
            if (this.blogData.length === 0) {
                this.loadBlog();
            }
        });

        tabActivity.addEventListener('click', () => {
            this.activateTab(tabActivity, [tabBlog, tabCommits, tabProjects]);
            this.showContent(activityTimeline, [blogTimeline, commitsTimeline, projectsTimeline]);
            if (this.activities.length === 0) {
                this.loadActivity();
            }
        });

        tabCommits.addEventListener('click', () => {
            this.activateTab(tabCommits, [tabBlog, tabActivity, tabProjects]);
            this.showContent(commitsTimeline, [blogTimeline, activityTimeline, projectsTimeline]);
            this.loadCommits(1);
        });

        tabProjects.addEventListener('click', () => {
            this.activateTab(tabProjects, [tabBlog, tabActivity, tabCommits]);
            this.showContent(projectsTimeline, [blogTimeline, activityTimeline, commitsTimeline]);
            this.loadProjects();
        });
    }

    activateTab(activeTab, inactiveTabs) {
        activeTab.classList.add('active');
        inactiveTabs.forEach(tab => tab.classList.remove('active'));
    }

    showContent(activeContent, inactiveContents) {
        activeContent.style.display = '';
        activeContent.classList.add('active');
        inactiveContents.forEach(content => {
            content.style.display = 'none';
            content.classList.remove('active');
        });
    }
    async loadBlog() {
        const blogTimeline = document.getElementById('blog-timeline');
        blogTimeline.innerHTML = '<div class="loading">Loading blog view...</div>';
        
        try {
            const response = await fetch('/api/blog');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            this.blogData = await response.json();
            this.renderBlog();
        } catch (error) {
            blogTimeline.innerHTML = `<div class="error">Failed to load blog view: ${error.message}</div>`;
        }
    }

    renderBlog() {
        const blogTimeline = document.getElementById('blog-timeline');
        if (!this.blogData || this.blogData.length === 0) {
            blogTimeline.innerHTML = '<div class="blog-entry">No blog data available. Click "Refresh Activity" to fetch your GitHub activity.</div>';
            return;
        }

        const content = this.blogData.map(entry => `
            <div class="blog-entry">
                <div class="blog-header">
                    <a href="${entry.url}" class="blog-repo-name" target="_blank">${entry.repository}</a>
                    <span class="blog-date">${this.formatDate(entry.latest_date)}</span>
                </div>
                
                ${entry.pull_requests && entry.pull_requests.length > 0 ? `
                    <div class="activity-section">
                        <h4 class="activity-section-title">📝 Pull Requests</h4>
                        <div class="activity-list">
                            ${entry.pull_requests.map(pr => `
                                <div class="activity-list-item">
                                    <span class="activity-date">${this.formatDate(pr.date)}</span>
                                    ${pr.url ? `<a href="${pr.url}" class="activity-link" target="_blank">
                                        ${pr.count > 1 ? `${pr.count} pull requests` : 'Pull request'}
                                    </a>` : `<span>${pr.count > 1 ? `${pr.count} pull requests` : 'Pull request'}</span>`}
                                </div>
                            `).join('')}
                        </div>
                    </div>
                ` : ''}
                
                ${entry.issues && entry.issues.length > 0 ? `
                    <div class="activity-section">
                        <h4 class="activity-section-title">🔧 Issues</h4>
                        <div class="activity-list">
                            ${entry.issues.map(issue => `
                                <div class="activity-list-item">
                                    <span class="activity-date">${this.formatDate(issue.date)}</span>
                                    ${issue.url ? `<a href="${issue.url}" class="activity-link" target="_blank">
                                        ${issue.count > 1 ? `${issue.count} issues` : 'Issue'}
                                    </a>` : `<span>${issue.count > 1 ? `${issue.count} issues` : 'Issue'}</span>`}
                                </div>
                            `).join('')}
                        </div>
                    </div>
                ` : ''}
                
                ${entry.commits && entry.commits.length > 0 ? `
                    <div class="activity-section">
                        <h4 class="activity-section-title">💻 Commits</h4>
                        <div class="activity-list">
                            ${entry.commits.map(commit => `
                                <div class="activity-list-item">
                                    <span class="activity-date">${this.formatDate(commit.date)}</span>
                                    ${commit.url ? `<a href="${commit.url}" class="activity-link" target="_blank">
                                        ${commit.count > 1 ? `${commit.count} commits` : 'Commit'}
                                    </a>` : `<span>${commit.count > 1 ? `${commit.count} commits` : 'Commit'}</span>`}
                                </div>
                            `).join('')}
                        </div>
                    </div>
                ` : ''}
            </div>
        `).join('');

        blogTimeline.innerHTML = content;
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

    async loadProjects() {
        const projectsTimeline = document.getElementById('projects-timeline');
        projectsTimeline.innerHTML = '<div class="loading">Loading project blog...</div>';
        
        try {
            const response = await fetch('/api/projects');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            this.projectsData = await response.json();
            this.renderProjects();
        } catch (error) {
            projectsTimeline.innerHTML = `<div class="error">Failed to load projects: ${error.message}</div>`;
        }
    }

    renderProjects() {
        const projectsTimeline = document.getElementById('projects-timeline');
        if (!this.projectsData || this.projectsData.length === 0) {
            projectsTimeline.innerHTML = '<div class="blog-entry">No project data available.</div>';
            return;
        }

        const content = this.projectsData.map(project => `
            <div class="blog-entry">
                <div class="blog-header">
                    <a href="${project.url}" class="blog-repo-name" target="_blank">${project.repository}</a>
                    <span class="blog-date">${this.formatDate(project.latest_date)}</span>
                </div>
                <div class="blog-stats">
                    <span class="stat-item">📊 ${project.total_commits} commits</span>
                    ${project.activity_types && project.activity_types.length > 0 ? 
                        `<span class="stat-item">🏷️ ${project.activity_types.join(', ')}</span>` : ''}
                </div>
                ${project.recent_comments && project.recent_comments.length > 0 ? `
                    <div class="blog-comments">
                        <h4 class="comments-header">💬 Recent PR Comments</h4>
                        ${project.recent_comments.map(comment => `
                            <div class="comment-item">
                                <div class="comment-header">
                                    <a href="${comment.pr_url}" class="pr-title" target="_blank">
                                        #${comment.pr_number} ${comment.pr_title}
                                    </a>
                                    <span class="comment-author">@${comment.author}</span>
                                </div>
                                <div class="comment-body">${this.truncateText(comment.body, 200)}</div>
                                <div class="comment-footer">
                                    <span class="comment-date">${this.formatDate(comment.created_at)}</span>
                                    <a href="${comment.comment_url}" class="comment-link" target="_blank">View comment →</a>
                                </div>
                            </div>
                        `).join('')}
                    </div>
                ` : '<div class="no-comments">No recent PR comments</div>'}
            </div>
        `).join('');

        projectsTimeline.innerHTML = content;
    }

    truncateText(text, maxLength) {
        if (!text) return '';
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
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
            
            // Reload all views in parallel
            await Promise.all([this.loadBlog(), this.loadActivity()]);
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