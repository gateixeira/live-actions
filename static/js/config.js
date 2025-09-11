const CONFIG = {
    PERIODS: { HOUR: 'hour', DAY: 'day', WEEK: 'week', MONTH: 'month' },
    PAGINATION: { DEFAULT_PAGE_SIZE: 10, MAX_VISIBLE_PAGES: 7, SIDE_PAGES: 2 },
    DURATIONS: { SECOND: 1000, MINUTE: 60000, HOUR: 3600000 }
};

let state = {
    dashboardData: null,
    currentPeriod: CONFIG.PERIODS.DAY,
    pagination: {
        currentPage: 1,
        currentPageSize: CONFIG.PAGINATION.DEFAULT_PAGE_SIZE,
        totalPages: 1,
        totalCount: 0
    },
    labelsPagination: {
        currentPage: 1,
        currentPageSize: CONFIG.PAGINATION.DEFAULT_PAGE_SIZE,
        totalPages: 1,
        totalCount: 0
    },
    labelMetrics: [],
    workflowRuns: [],
    workflowJobs: new Map(), // Map of runId -> jobs array
    runnerTypes: []
};

const statuses = {
    'queued': {
        class: 'status-queued',
        icon: '⏳',
        text: 'Queued'
    },
    'in_progress': {
        class: 'status-in-progress',
        icon: '🔄',
        text: 'Running'
    },
    'success': {
        class: 'status-success',
        icon: '✅',
        text: 'Success'
    },
    'failure': {
        class: 'status-failure',
        icon: '❌',
        text: 'Failed'
    },
    'cancelled': {
        class: 'status-cancelled',
        icon: '⏹️',
        text: 'Cancelled',
    },
    'skipped': {
        class: 'status-cancelled',
        icon: '⏭️',
        text: 'Skipped',
    },
    'requested': {
        class: 'status-requested',
        icon: '📝',
        text: 'Requested'
    },
    'action_required': {
        class: 'status-action-required',
        icon: '⚠️',
        text: 'Needs Approval'
    },
    'waiting': {
        class: 'status-action-required',
        icon: '⚠️',
        text: 'Needs Approval'
    },
    
};

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

const utils = {
    formatRelativeTime(date) {
        const now = new Date();
        const diffMs = now - date;
        const diffSeconds = Math.floor(diffMs / 1000);
        const diffMinutes = Math.floor(diffSeconds / 60);
        const diffHours = Math.floor(diffMinutes / 60);
        const diffDays = Math.floor(diffHours / 24);
        
        if (diffSeconds < 60) return diffSeconds <= 1 ? 'just now' : `${diffSeconds}s ago`;
        if (diffMinutes < 60) return diffMinutes === 1 ? '1 min ago' : `${diffMinutes} mins ago`;
        if (diffHours < 24) return diffHours === 1 ? '1 hour ago' : `${diffHours} hours ago`;
        if (diffDays < 7) return diffDays === 1 ? '1 day ago' : `${diffDays} days ago`;
        
        return date.toLocaleDateString([], { 
            month: 'short', 
            day: 'numeric',
            year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined
        });
    },
    
    calculateDuration(startedAt, status, endTime) {
        if (!startedAt) return '-';
        
        const start = new Date(startedAt);
        const end = status.toLowerCase() === 'in_progress' ? new Date() : 
                  (status.toLowerCase() === 'completed' && endTime ? new Date(endTime) : null);
        
        if (!end) return '-';
        
        const duration = end - start;
        if (duration < CONFIG.DURATIONS.SECOND) return '< 1s';
        if (duration < CONFIG.DURATIONS.MINUTE) return `${Math.floor(duration / 1000)}s`;
        if (duration < CONFIG.DURATIONS.HOUR) return `${Math.floor(duration / CONFIG.DURATIONS.MINUTE)}m ${Math.floor((duration % CONFIG.DURATIONS.MINUTE) / 1000)}s`;
        
        const hours = Math.floor(duration / CONFIG.DURATIONS.HOUR);
        const minutes = Math.floor((duration % CONFIG.DURATIONS.HOUR) / CONFIG.DURATIONS.MINUTE);
        return `${hours}h ${minutes}m`;
    },
    
    // Format seconds to human-readable time format (e.g., 941.5 -> "16m42s")
    formatSecondsToTime(seconds) {
        if (!seconds || seconds === 0) return '0s';
        
        const totalSeconds = Math.round(seconds);
        if (totalSeconds < 60) return `${totalSeconds}s`;
        
        const minutes = Math.floor(totalSeconds / 60);
        const remainingSeconds = totalSeconds % 60;
        
        if (minutes < 60) {
            return remainingSeconds > 0 ? `${minutes}m${remainingSeconds}s` : `${minutes}m`;
        }
        
        const hours = Math.floor(minutes / 60);
        const remainingMinutes = minutes % 60;
        
        let result = `${hours}h`;
        if (remainingMinutes > 0) result += `${remainingMinutes}m`;
        if (remainingSeconds > 0 && remainingMinutes === 0) result += `${remainingSeconds}s`;
        
        return result;
    },
    
    getCookie(name) {
        const value = `; ${document.cookie}`;
        const parts = value.split(`; ${name}=`);
        return parts.length === 2 ? parts.pop().split(';').shift() : null;
    },
    
    // Generate colors for dynamic runner types
    generateColors() {
        const computedStyle = getComputedStyle(document.documentElement);
        const baseColors = {
            text: computedStyle.getPropertyValue('--color-chart-text').trim(),
            grid: computedStyle.getPropertyValue('--color-chart-grid').trim(),
            total: computedStyle.getPropertyValue('--color-chart-total').trim(),
            queued: computedStyle.getPropertyValue('--color-chart-queued').trim()
        };
        
        const knownTypeColors = {
            'github-hosted': computedStyle.getPropertyValue('--color-chart-github-hosted').trim(),
            'self-hosted': computedStyle.getPropertyValue('--color-chart-self-hosted').trim(),
            'unknown': computedStyle.getPropertyValue('--color-chart-unknown').trim()
        };
            
        return { ...baseColors, knownTypeColors };
    },
    
    getRunnerTypeColor(runnerType, colors) {
        // Return the specific color if it exists, otherwise fall back to the unknown color
        return colors.knownTypeColors[runnerType] || colors.knownTypeColors['unknown'];
    },
    
    formatRunnerTypeLabel(runnerType) {
        switch (runnerType) {
            case 'github-hosted':
                return 'GitHub-hosted';
            case 'self-hosted':
                return 'Self-hosted';
            default:
                return runnerType.charAt(0).toUpperCase() + runnerType.slice(1);
        }
    },
};

function getStatus(status, conclusion, property) {
    const statusData = statuses[conclusion] || statuses[status];
    const defaultStatus = { class: 'status-unknown', icon: '❓', text: 'Unknown' };
    return (statusData || defaultStatus)[property];
}
