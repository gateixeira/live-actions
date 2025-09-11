// Dashboard Application - Runners Demand Monitoring
document.addEventListener('DOMContentLoaded', function() {
    function initializeTheme() {
        const darkThemeBtn = document.getElementById('darkThemeBtn');
        const lightThemeBtn = document.getElementById('lightThemeBtn');

        const savedTheme = localStorage.getItem('theme') || 'dark';
        setTheme(savedTheme);
        
        darkThemeBtn.addEventListener('click', function() {
            setTheme('dark');
            localStorage.setItem('theme', 'dark');
            updateChartAfterThemeChange();
        });
        
        lightThemeBtn.addEventListener('click', function() {
            setTheme('light');
            localStorage.setItem('theme', 'light');
            updateChartAfterThemeChange();
        });
        
        function setTheme(theme) {
            document.documentElement.setAttribute('data-theme', theme);
            
            if (theme === 'light') {
                lightThemeBtn.classList.add('active');
                darkThemeBtn.classList.remove('active');
            } else {
                darkThemeBtn.classList.add('active');
                lightThemeBtn.classList.remove('active');
            }
        }
        
        function updateChartAfterThemeChange() {
            if (chartManager.chart) {
                setTimeout(() => {
                    chartManager.updateChart();
                }, 50);
            }
        }
    }
    
    initializeTheme();
    
    let eventSource = null;
    
    function initializeSSE() {
        connectSSE();
    }
    
    function connectSSE() {
        if (eventSource) {
            eventSource.close();
        }
        
        eventSource = new EventSource('/events');
        
        eventSource.onopen = () => {
            console.log('SSE connection opened');
        };
        
        eventSource.onmessage = (event) => {
            const data = JSON.parse(event.data);
            handleSSEEvent(data);
        };
        
        eventSource.onerror = (error) => {
            console.error('SSE connection error:', error);
        };
    }
    
    function handleSSEEvent(event) {
        switch (event.type) {
            case 'connected':
                console.log('SSE connected:', event.data.message);
                break;
                
            case 'metrics_update':
                updateMetricsDisplay(event.data);
                break;
                
            case 'workflow_update':
                handleWorkflowUpdate(event.data);
                break;
                
            default:
                console.log('Unknown SSE event type:', event.type);
        }
    }
    
    function updateMetricsDisplay(data) {
        const currentCount = document.getElementById('currentCount');
        const currentQueuedCount = document.getElementById('currentQueuedCount');
        const avgQueueTime = document.getElementById('avgQueueTime');
        const peakDemand = document.getElementById('peakDemand');
        const peakDemandTimestamp = document.getElementById('peakDemandTimestamp');
        
        if (currentCount) currentCount.textContent = Math.round(data.running_jobs || 0);
        if (currentQueuedCount) currentQueuedCount.textContent = Math.round(data.queued_jobs || 0);
        
        if (avgQueueTime) {
            const queueTimeSeconds = data.avg_queue_time;
            if (queueTimeSeconds !== undefined && queueTimeSeconds !== null) {
                avgQueueTime.textContent = utils.formatSecondsToTime(queueTimeSeconds);
            }
        }
        if (peakDemand) peakDemand.textContent = Math.round(data.peak_demand || peakDemand.textContent);
        
        if (peakDemandTimestamp && data.peak_timestamp) {
            const date = new Date(data.peak_timestamp);
            peakDemandTimestamp.textContent = utils.formatRelativeTime(date);
        }

        if (data.label_metrics && Array.isArray(data.label_metrics)) {
            labelManager.updateLabelMetricsInline(data.label_metrics);
        }
    }

    function handleWorkflowUpdate(data) {
        const workflowRun = data.workflow_run;
        workflowManager.updateWorkflowRunInline(workflowRun);
    }
    
    function initializeDashboard() {
        
        const dashboardDataElement = document.getElementById('dashboard-data');
        state.dashboardData = JSON.parse(dashboardDataElement.textContent);

        chartManager.init();
        workflowManager.init();
        labelManager.init();
        
        initializeSSE();
    }
    
    const chartManager = {
        chart: null,
        refreshInterval: null,
        
        init() {
            this.initializePeriodControls();
            this.updateChart();
            setInterval(() => {
                this.fetchData();
            }, 30000);
        },
        
        initializePeriodControls() {
            const periodButtons = document.querySelectorAll('[data-period]');
            const activePeriodBtn = document.querySelector('.btn-toolbar .btn.active[data-period]');
            
            if (activePeriodBtn) {
                state.currentPeriod = activePeriodBtn.dataset.period;
            }
            
            periodButtons.forEach(button => {
                button.addEventListener('click', () => {
                    periodButtons.forEach(btn => btn.classList.remove('active'));
                    button.classList.add('active');
                    state.currentPeriod = button.dataset.period;
                    this.fetchData();
                });
            });
        },
        
        async fetchData() {
            const { start, end, step } = this.getTimeRangeForPeriod(state.currentPeriod);
            
            const url = new URL('/api/metrics/query_range', window.location.origin);
            url.searchParams.set('period', state.currentPeriod);
            url.searchParams.set('start', Math.floor(start.getTime() / 1000).toString());
            url.searchParams.set('end', Math.floor(end.getTime() / 1000).toString());
            url.searchParams.set('step', step);
            
            try {
                const response = await fetch(url, {
                    headers: {
                        'Referer': window.location.href,
                        'X-Requested-With': 'XMLHttpRequest',
                        'X-CSRF-Token': utils.getCookie('csrf_token') || state.dashboardData.csrfToken
                    }
                });
                
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                
                const data = await response.json();
                
                if (data.current_metrics) {
                    updateMetricsDisplay(data.current_metrics);
                }
                
                if (data.time_series && data.time_series.running_jobs && data.time_series.queued_jobs) {
                    const historicalData = this.convertSeparateTimeSeries(
                        data.time_series.running_jobs.data.result || [],
                        data.time_series.queued_jobs.data.result || []
                    );
                    this.updateChart(historicalData);
                } else {
                    console.log('No time series data available');
                    this.updateChart([]);
                }
            } catch (error) {
                console.error('Failed to fetch metrics data:', error);
                this.updateChart([]);
            }
        },
        
        updateChart(historicalData = null) {
            if (!historicalData) {
                this.fetchData();
                return;
            }
            
            if (historicalData.length === 0) {
                console.log('No historical data to display');
                const canvas = document.getElementById('demandChart');
                if (canvas) {
                    canvas.style.display = 'block';
                    const ctx = canvas.getContext('2d');
                    const colors = utils.generateColors();
                    
                    if (this.chart) {
                        this.chart.destroy();
                    }
                    
                    this.chart = new Chart(ctx, {
                        type: 'line',
                        data: {
                            labels: [],
                            datasets: []
                        },
                        options: {
                            responsive: true,
                            maintainAspectRatio: false,
                            plugins: {
                                legend: {
                                    labels: { color: colors.text }
                                }
                            },
                            scales: {
                                y: {
                                    beginAtZero: true,
                                    ticks: { stepSize: 1, color: colors.text },
                                    grid: { color: colors.grid }
                                },
                                x: {
                                    ticks: { color: colors.text },
                                    grid: { color: colors.grid }
                                }
                            }
                        }
                    });
                }
                return;
            }
            
            const canvas = document.getElementById('demandChart');
            if (!canvas) {
                console.error('Canvas element not found');
                return;
            }

            canvas.style.display = 'block';
            
            const ctx = canvas.getContext('2d');
            const colors = utils.generateColors();
            
            if (this.chart) this.chart.destroy();
            
            const datasets = [];
            
            state.runnerTypes.forEach((runnerType) => {
                const color = utils.getRunnerTypeColor(runnerType, colors);
                const label = utils.formatRunnerTypeLabel(runnerType) + ' (Running)';
                
                datasets.push({
                    label: label,
                    data: historicalData.map(entry => entry.runnerTypeCounts[runnerType] || 0),
                    borderColor: color,
                    backgroundColor: color + '20',
                    tension: 0.1,
                    fill: false
                });
            });
            
            state.runnerTypes.forEach((runnerType) => {
                const color = utils.getRunnerTypeColor(runnerType, colors);
                const label = utils.formatRunnerTypeLabel(runnerType) + ' (Queued)';
                
                datasets.push({
                    label: label,
                    data: historicalData.map(entry => entry.queuedByRunnerType[runnerType] || 0),
                    borderColor: color,
                    backgroundColor: color + '40',
                    tension: 0.1,
                    fill: false,
                    borderDash: [5, 5] // Dashed line for queued jobs
                });
            });
            
            datasets.push({
                label: 'Total Jobs',
                data: historicalData.map(entry => entry.total_count),
                borderColor: colors.total,
                backgroundColor: colors.total + '20',
                tension: 0.1,
                fill: false,
                borderWidth: 3
            });
            
            this.chart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: historicalData.map(entry => this.formatTimestamp(new Date(entry.timestamp))),
                    datasets: datasets
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            labels: {
                                color: colors.text,
                                usePointStyle: true,
                                padding: 20
                            }
                        }
                    },
                    scales: {
                        y: {
                            beginAtZero: true,
                            ticks: { stepSize: 1, color: colors.text },
                            grid: { color: colors.grid }
                        },
                        x: {
                            ticks: { color: colors.text },
                            grid: { color: colors.grid }
                        }
                    }
                }});

            window.runnersChart = this.chart;
        },
        
        convertSeparateTimeSeries(runningJobsResult, queuedJobsResult) {
            const timeMap = new Map();
            const runnerTypes = new Set();
            
            if (runningJobsResult && Array.isArray(runningJobsResult)) {
                runningJobsResult.forEach(series => {
                    if (series.metric && series.metric.runner_type && series.values) {
                        const runnerType = series.metric.runner_type;
                        runnerTypes.add(runnerType);
                        
                        series.values.forEach(([timestamp, value]) => {
                            const time = new Date(timestamp * 1000);
                            const isoKey = time.toISOString();
                            
                            if (!timeMap.has(isoKey)) {
                                timeMap.set(isoKey, {
                                    timestamp: time,
                                    runnerTypeCounts: {},
                                    queuedByRunnerType: {}
                                });
                            }
                            
                            const entry = timeMap.get(isoKey);
                            const numValue = parseFloat(value) || 0;
                            entry.runnerTypeCounts[runnerType] = numValue;
                        });
                    }
                });
            }
            
            if (queuedJobsResult && Array.isArray(queuedJobsResult)) {
                queuedJobsResult.forEach(series => {
                    if (series.metric && series.metric.runner_type && series.values) {
                        const runnerType = series.metric.runner_type;
                        runnerTypes.add(runnerType);
                        
                        series.values.forEach(([timestamp, value]) => {
                            const time = new Date(timestamp * 1000);
                            const isoKey = time.toISOString();
                            
                            if (!timeMap.has(isoKey)) {
                                timeMap.set(isoKey, {
                                    timestamp: time,
                                    runnerTypeCounts: {},
                                    queuedByRunnerType: {}
                                });
                            }
                            
                            const entry = timeMap.get(isoKey);
                            const numValue = parseFloat(value) || 0;
                            entry.queuedByRunnerType[runnerType] = numValue;
                        });
                    }
                });
            }
            
            state.runnerTypes = Array.from(runnerTypes).sort();
            
            return Array.from(timeMap.values())
                .sort((a, b) => a.timestamp - b.timestamp)
                .map(entry => {
                    const result = {
                        timestamp: entry.timestamp,
                        count_queued: Object.values(entry.queuedByRunnerType).reduce((sum, count) => sum + count, 0),
                        runnerTypeCounts: entry.runnerTypeCounts,
                        queuedByRunnerType: entry.queuedByRunnerType
                    };
                    
                    result.total_running = Object.values(entry.runnerTypeCounts).reduce((sum, count) => sum + count, 0);
                    result.total_count = result.total_running + result.count_queued;
                    
                    return result;
                });
        },

        convertPrometheusData(combinedResult) {
            const timeMap = new Map();
            const runnerTypes = new Set();
            
            if (combinedResult && Array.isArray(combinedResult)) {
                combinedResult.forEach(series => {
                    if (series.metric && series.metric.runner_type && series.metric.job_status && series.values) {
                        const runnerType = series.metric.runner_type;
                        const jobStatus = series.metric.job_status;
                        runnerTypes.add(runnerType);
                        
                        series.values.forEach(([timestamp, value]) => {
                            const time = new Date(timestamp * 1000);
                            const isoKey = time.toISOString();
                            
                            if (!timeMap.has(isoKey)) {
                                timeMap.set(isoKey, {
                                    timestamp: time,
                                    count_queued: 0,
                                    runnerTypeCounts: {},
                                    queuedByRunnerType: {}
                                });
                            }
                            
                            const entry = timeMap.get(isoKey);
                            const numValue = parseFloat(value) || 0;
                            
                            if (jobStatus === 'running') {
                                entry.runnerTypeCounts[runnerType] = numValue;
                            } else if (jobStatus === 'queued') {
                                entry.queuedByRunnerType[runnerType] = numValue;
                            }
                        });
                    }
                });
            }
            
            state.runnerTypes = Array.from(runnerTypes).sort();
            
            return Array.from(timeMap.values())
                .sort((a, b) => a.timestamp - b.timestamp)
                .map(entry => {
                    const result = {
                        timestamp: entry.timestamp,
                        count_queued: Object.values(entry.queuedByRunnerType).reduce((sum, count) => sum + count, 0),
                        runnerTypeCounts: entry.runnerTypeCounts,
                        queuedByRunnerType: entry.queuedByRunnerType
                    };
                    
                    result.total_running = Object.values(entry.runnerTypeCounts).reduce((sum, count) => sum + count, 0);
                    result.total_count = result.total_running + result.count_queued;
                    
                    return result;
                });
        },
        
        getTimeRangeForPeriod(period) {
            const now = new Date();
            const start = new Date();
            let step = '60s';
            
            switch (period) {
                case CONFIG.PERIODS.HOUR:
                    start.setHours(now.getHours() - 1);
                    step = '30s';
                    break;
                case CONFIG.PERIODS.WEEK:
                    start.setDate(now.getDate() - 7);
                    step = '30m';
                    break;
                case CONFIG.PERIODS.MONTH:
                    start.setMonth(now.getMonth() - 1);
                    step = '2h';
                    break;
                default: // day
                    start.setDate(now.getDate() - 1);
                    step = '5m';
            }
            
            return { start, end: now, step };
        },
        
        formatTimestamp(date) {
            switch (state.currentPeriod) {
                case CONFIG.PERIODS.HOUR:
                case CONFIG.PERIODS.DAY:
                    return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit', hour12: false });
                case CONFIG.PERIODS.WEEK:
                    return date.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric' });
                case CONFIG.PERIODS.MONTH:
                    return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
                default:
                    return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit', hour12: false });
            }
        },
    };
    
    const workflowManager = {
        durationUpdateInterval: null,

        init() {
            this.initializePaginationHandlers();
            this.loadWorkflowRuns();
            this.startDurationCounter();
        },

        startDurationCounter() {
            if (this.durationUpdateInterval) {
                clearInterval(this.durationUpdateInterval);
            }

            this.durationUpdateInterval = setInterval(() => {
                this.updateRunningWorkflowDurations();
            }, 1000);
        },

        updateRunningWorkflowDurations() {
            const tableBody = document.getElementById('workflowRunsTableBody');
            if (!tableBody) return;

            const runRows = tableBody.querySelectorAll('tr[data-run-id]');
            
            runRows.forEach(row => {
                const runId = parseInt(row.dataset.runId);
                const workflowRun = state.workflowRuns.find(run => run.id === runId);
                
                // Update workflow run duration if in progress
                if (workflowRun && workflowRun.status === 'in_progress' && workflowRun.run_started_at) {
                    const durationCell = row.querySelector('.time-cell');
                    if (durationCell) {
                        const currentDuration = utils.calculateDuration(
                            workflowRun.run_started_at, 
                            workflowRun.status, 
                            null
                        );
                        durationCell.textContent = currentDuration;
                    }
                }
                
                // Update job durations if jobs are expanded and in progress
                const jobsRow = row.nextElementSibling;
                if (jobsRow?.classList.contains('workflow-jobs-row')) {
                    this.updateRunningJobDurations(runId, jobsRow);
                }
            });
        },

        updateRunningJobDurations(runId, jobsRow) {
            const jobs = state.workflowJobs.get(runId);
            if (!jobs) return;

            const jobRows = jobsRow.querySelectorAll('tr[data-job-id]');
            
            jobRows.forEach(jobRow => {
                const jobId = parseInt(jobRow.dataset.jobId);
                const job = jobs.find(j => j.id === jobId);
                
                if (job && job.status === 'in_progress' && job.started_at) {
                    const durationCell = jobRow.querySelector('.job-duration-cell');
                    if (durationCell) {
                        const currentDuration = utils.calculateDuration(
                            job.started_at,
                            job.status,
                            null
                        );
                        durationCell.textContent = currentDuration;
                    }
                }
            });
        },

        stopDurationCounter() {
            if (this.durationUpdateInterval) {
                clearInterval(this.durationUpdateInterval);
                this.durationUpdateInterval = null;
            }
        },

        async loadWorkflowRuns(page = 1, pageSize = null) {
            const elements = {
                table: document.getElementById('workflowRunsTable'),
                tableBody: document.getElementById('workflowRunsTableBody'),
                pagination: document.getElementById('paginationSection')
            };

            state.pagination.currentPage = page;
            if (pageSize !== null) state.pagination.currentPageSize = pageSize;

            elements.table?.classList.add('hidden');
            if (elements.pagination) elements.pagination.style.display = 'none';
            if (elements.tableBody) elements.tableBody.innerHTML = '';
            
            
            const url = new URL('/api/workflow-runs', window.location.origin);
            url.searchParams.set('page', state.pagination.currentPage.toString());
            url.searchParams.set('limit', state.pagination.currentPageSize.toString());
            
            const response = await fetch(url, {
                headers: {
                    'Referer': window.location.href,
                    'X-Requested-With': 'XMLHttpRequest',
                    'X-CSRF-Token': utils.getCookie('csrf_token') || state.dashboardData.csrfToken
                }
            });
            
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            
            const data = await response.json();
            const workflowRuns = data.workflow_runs || [];
            
            state.pagination.totalPages = data.pagination?.total_pages || 1;
            state.pagination.totalCount = data.pagination?.total_count || 0;
            state.pagination.currentPage = data.pagination?.current_page || 1;
            
            state.workflowRuns = workflowRuns.slice();
            
            // Clear job state when loading new workflow runs to prevent memory leaks
            state.workflowJobs.clear();

            elements.table?.classList.remove('hidden');

            if (workflowRuns.length === 0 && state.pagination.currentPage === 1) {
                if (elements.tableBody) {
                    elements.tableBody.innerHTML = '<tr><td colspan="8" class="empty-state">No workflow runs found</td></tr>';
                }
                if (elements.pagination) elements.pagination.style.display = 'none';
                return;
            }
            
            if (state.pagination.totalCount > 0) {
                if (elements.pagination) elements.pagination.style.display = 'flex';
                this.updatePaginationControls();
            }
            
            workflowRuns.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
            
            if (elements.tableBody) {
                elements.tableBody.innerHTML = workflowRuns.map(run => this.createWorkflowRunRow(run)).join('');
            }

            this.startDurationCounter();
        },
        
        createWorkflowRunRow(run) {
            const createdAt = new Date(run.created_at);
            const duration = utils.calculateDuration(run.run_started_at, run.status, run.updated_at);
            
            return `
                <tr data-run-id="${run.id}">
                    <td class="dropdown-cell">
                        <button class="dropdown-toggle" onclick="workflowManager.toggleWorkflowJobs(${run.id}, this)">
                            <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
                                <path d="M6 9l3-3H3l3 3z"/>
                            </svg>
                        </button>
                    </td>
                    <td class="id-cell">#${run.id}</td>
                    <td class="workflow-cell">${run.name}</td>
                    <td class="repo-cell">${run.repository_name}</td>
                    <td class="build-title-cell">
                        <a href="${run.html_url}" target="_blank">${run.display_title}</a>
                    </td>
                    <td class="build-status-cell">
                        <div class="status-indicator ${getStatus(run.status, run.conclusion, 'class')}">
                            <span class="status-icon">${getStatus(run.status, run.conclusion, 'icon')}</span>
                            <span>${getStatus(run.status, run.conclusion, 'text')}</span>
                        </div>
                    </td>
                    <td class="time-cell">${duration}</td>
                    <td class="time-cell">${utils.formatRelativeTime(createdAt)}</td>
                </tr>
            `;
        },
        
        async toggleWorkflowJobs(runId, button) {
            const runRow = button.closest('tr');
            const existingJobsRow = runRow.nextElementSibling;
            
            if (existingJobsRow?.classList.contains('workflow-jobs-row')) {
                existingJobsRow.remove();
                button.classList.remove('expanded');
                // Keep jobs in state for potential re-expansion, don't remove them
                return;
            }
            
            button.classList.add('expanded');
            
           
            const jobs = await this.fetchWorkflowJobs(runId);
            const jobsRow = this.createWorkflowJobsRow(jobs, runRow.children.length);
            runRow.insertAdjacentElement('afterend', jobsRow);
        },
        
        async fetchWorkflowJobs(runId) {
            const url = new URL(`/api/workflow-jobs/${runId}`, window.location.origin);
            
            const response = await fetch(url, {
                headers: {
                    'Referer': window.location.href,
                    'X-Requested-With': 'XMLHttpRequest',
                    'X-CSRF-Token': utils.getCookie('csrf_token') || state.dashboardData.csrfToken
                }
            });
            
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            
            const data = await response.json();
            const jobs = data.workflow_jobs || [];
            
            // Store jobs in state for duration calculation
            state.workflowJobs.set(runId, jobs);
            
            return jobs;
        },
        
        createWorkflowJobsRow(jobs, colSpan) {
            const jobsRow = document.createElement('tr');
            jobsRow.className = 'workflow-jobs-row';
            
            let jobsTable = '<div class="jobs-container">';
            
            if (jobs.length === 0) {
                jobsTable += '<p class="no-jobs">No workflow jobs found for this run.</p>';
            } else {
                jobsTable += `
                    <table class="jobs-table">
                        <thead>
                            <tr>
                                <th>Job ID</th>
                                <th>Name</th>
                                <th>Status</th>
                                <th>Runner Type</th>
                                <th>Labels</th>
                                <th>Duration</th>
                                <th>Created</th>
                            </tr>
                        </thead>
                        <tbody>
                `;
                
                jobs.forEach(job => {
                    const createdAt = new Date(job.created_at);
                    const jobDuration = utils.calculateDuration(job.started_at, job.status, job.completed_at);
                    const labelsDisplay = job.labels?.length > 0 ? job.labels.join(', ') : 'None';
                    
                    jobsTable += `
                        <tr data-job-id="${job.id}">
                            <td class="job-id-cell">#${job.id}</td>
                            <td class="job-name-cell">${job.name}</td>
                            <td class="job-status-cell">
                                <div class="status-indicator ${getStatus(job.status, job.conclusion, 'class')}">
                                    <span class="status-icon">${getStatus(job.status, job.conclusion, 'icon')}</span>
                                    <span>${getStatus(job.status, job.conclusion, 'text')}</span>
                                </div>
                            </td>
                            <td class="runner-type-cell">${job.runner_type}</td>
                            <td class="labels-cell">${labelsDisplay}</td>
                            <td class="job-duration-cell">${jobDuration}</td>
                            <td class="job-created-cell">${utils.formatRelativeTime(createdAt)}</td>
                        </tr>
                    `;
                });
                
                jobsTable += '</tbody></table>';
            }
            
            jobsTable += '</div>';
            
            jobsRow.innerHTML = `<td colspan="${colSpan}" class="jobs-cell">${jobsTable}</td>`;
            return jobsRow;
        },
        
        initializePaginationHandlers() {
            const config = {
                pageSizeId: 'pageSize',
                firstId: 'firstPage',
                prevId: 'prevPage',
                nextId: 'nextPage',
                lastId: 'lastPage',
                paginationState: state.pagination,
                loadFunction: (page, pageSize) => this.loadWorkflowRuns(page, pageSize)
            };
            
            paginationUtils.initializeHandlers(config);
        },
        
        updatePaginationControls() {
            const config = {
                infoId: 'paginationInfo',
                firstId: 'firstPage',
                prevId: 'prevPage',
                nextId: 'nextPage',
                lastId: 'lastPage',
                pageNumbersId: 'pageNumbers',
                paginationState: state.pagination,
                loadFunction: (page) => this.loadWorkflowRuns(page),
                itemName: 'workflow runs'
            };
            
            paginationUtils.updateControls(config);
        },
        
        updateWorkflowRunInline(workflowRun) {
            if (state.pagination.currentPage !== 1) {
                return false;
            }
            
            const existingIndex = state.workflowRuns.findIndex(run => run.id === workflowRun.id);
            let updated = false;
            
            if (existingIndex !== -1) {
                state.workflowRuns[existingIndex] = { ...state.workflowRuns[existingIndex], ...workflowRun };
                this.updateWorkflowRunRowInDOM(workflowRun);
                updated = true;
            } else {
                state.workflowRuns.unshift(workflowRun);
                
                if (state.workflowRuns.length > state.pagination.currentPageSize) {
                    state.workflowRuns.pop();
                }
                
                state.pagination.totalCount++;
                state.pagination.totalPages = Math.ceil(state.pagination.totalCount / state.pagination.currentPageSize);

                this.renderWorkflowRunsTable();
                this.updatePaginationControls();
                updated = true;
            }
            
            if (updated) {
                this.showUpdateFeedback();
                this.startDurationCounter();
            }
            
            return updated;
        },
        
        updateWorkflowRunRowInDOM(workflowRun) {
            const tableBody = document.getElementById('workflowRunsTableBody');
            const runRow = tableBody?.querySelector(`tr[data-run-id="${workflowRun.id}"]`);
            
            if (runRow) {
                const statusCell = runRow.querySelector('.build-status-cell .status-indicator');
                if (statusCell && workflowRun.status) {
                    const statusClass = getStatus(workflowRun.status, workflowRun.conclusion, 'class');
                    const statusIcon = getStatus(workflowRun.status, workflowRun.conclusion, 'icon');
                    const statusText = getStatus(workflowRun.status, workflowRun.conclusion, 'text');
                    
                    statusCell.className = `status-indicator ${statusClass}`;
                    statusCell.querySelector('.status-icon').textContent = statusIcon;
                    statusCell.querySelector('span:not(.status-icon)').textContent = statusText;
                }
                
                const timeCells = runRow.querySelectorAll('.time-cell');
                if (timeCells[0] && workflowRun.run_started_at) {
                    const duration = utils.calculateDuration(workflowRun.run_started_at, workflowRun.status, workflowRun.updated_at);
                    timeCells[0].textContent = duration;
                }
            }
        },
        
        renderWorkflowRunsTable() {
            const tableBody = document.getElementById('workflowRunsTableBody');
            if (tableBody) {
                const sortedRuns = [...state.workflowRuns].sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
                tableBody.innerHTML = sortedRuns.map(run => this.createWorkflowRunRow(run)).join('');
                this.startDurationCounter();
            }
        },
        
        showUpdateFeedback() {
            const table = document.getElementById('workflowRunsTable');
            const timestamp = document.getElementById('workflowRunsLastUpdated');
            
            if (table) {
                table.style.transition = 'background-color 0.3s ease';
                table.style.backgroundColor = 'var(--color-accent-subtle)';
                setTimeout(() => {
                    table.style.backgroundColor = '';
                    setTimeout(() => {
                        table.style.transition = '';
                    }, 300);
                }, 500);
            }
            
            if (timestamp) {
                const now = new Date();
                timestamp.textContent = `Updated at ${now.toLocaleTimeString()}`;
            }
        },
    };
    
    const labelManager = {
        init() {
            this.initializeLabelsPaginationHandlers();
            this.loadLabelMetrics();
        },
        
        initializeLabelsPaginationHandlers() {
            const config = {
                pageSizeId: 'labelsPageSize',
                firstId: 'labelsFirstPage',
                prevId: 'labelsPrevPage',
                nextId: 'labelsNextPage',
                lastId: 'labelsLastPage',
                paginationState: state.labelsPagination,
                loadFunction: (page, pageSize) => this.loadLabelMetrics(page, pageSize)
            };
            
            paginationUtils.initializeHandlers(config);
        },
        
        async loadLabelMetrics(page = 1, pageSize = null) {
            const elements = {
                table: document.getElementById('labelsTable'),
                tableBody: document.getElementById('labelsTableBody'),
                pagination: document.getElementById('labelsPaginationSection')
            };

            state.labelsPagination.currentPage = page;
            if (pageSize !== null) state.labelsPagination.currentPageSize = pageSize;

            if (elements.table) elements.table.style.display = 'none';

            const url = new URL('/api/label-metrics', window.location.origin);
            url.searchParams.set('page', state.labelsPagination.currentPage.toString());
            url.searchParams.set('limit', state.labelsPagination.currentPageSize.toString());
            
            const response = await fetch(url, {
                headers: {
                    'Referer': window.location.href,
                    'X-Requested-With': 'XMLHttpRequest',
                    'X-CSRF-Token': utils.getCookie('csrf_token') || state.dashboardData.csrfToken
                }
            });
            
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            
            const data = await response.json();
            state.labelMetrics = data.label_metrics || [];

            state.labelsPagination.totalPages = data.pagination?.total_pages || 1;
            state.labelsPagination.totalCount = data.pagination?.total_count || 0;
            state.labelsPagination.currentPage = data.pagination?.current_page || 1;

            if (elements.table) elements.table.style.display = 'table';

            if (state.labelMetrics.length === 0 && state.labelsPagination.currentPage === 1) {
                if (elements.tableBody) {
                    elements.tableBody.innerHTML = '<tr><td colspan="6" class="empty-state">No label metrics found</td></tr>';
                }
                if (elements.pagination) elements.pagination.style.display = 'none';
                return;
            }

            if (state.labelsPagination.totalCount > 0) {
                if (elements.pagination) elements.pagination.style.display = 'flex';
                this.updateLabelsPagination();
            }

            if (elements.tableBody) {
                elements.tableBody.innerHTML = state.labelMetrics.map(metric => `
                    <tr>
                        <td class="table-name-cell">${metric.labels}</td>
                        <td class="table-stat-value">${metric.running_count}</td>
                        <td class="table-stat-value">${metric.queued_count}</td>
                        <td class="table-stat-value">${metric.total_count}</td>
                        <td class="table-stat-value">${metric.completed_count}</td>
                        <td>
                            <div class="table-type-badge ${metric.runner_type}">
                                <span class="table-type-icon ${metric.runner_type}"></span>
                                ${metric.runner_type === 'github-hosted' ? 'GitHub-Hosted' : 
                                  metric.runner_type === 'self-hosted' ? 'Self-Hosted' : 'Unknown'}
                            </div>
                        </td>
                    </tr>
                `).join('');
            }
        },

        updateLabelsPagination() {
            const config = {
                infoId: 'labelsPaginationInfo',
                firstId: 'labelsFirstPage',
                prevId: 'labelsPrevPage',
                nextId: 'labelsNextPage',
                lastId: 'labelsLastPage',
                pageNumbersId: 'labelsPageNumbers',
                paginationState: state.labelsPagination,
                loadFunction: (page) => this.loadLabelMetrics(page),
                itemName: 'label metrics'
            };
            
            paginationUtils.updateControls(config);
        },
        
        updateLabelMetricsInline(labelMetrics) {
            if (state.labelsPagination.currentPage !== 1) {
                return false;
            }
            
            state.labelMetrics = labelMetrics.slice(0, 10);

            this.renderLabelMetricsTable();            
            this.showUpdateFeedback();
            
            return true;
        },
        
        showUpdateFeedback() {
            const table = document.getElementById('labelsTable');
            const timestamp = document.getElementById('labelsLastUpdated');
            
            if (table) {
                table.style.transition = 'background-color 0.3s ease';
                table.style.backgroundColor = 'var(--color-accent-subtle)';
                setTimeout(() => {
                    table.style.backgroundColor = '';
                    setTimeout(() => {
                        table.style.transition = '';
                    }, 300);
                }, 500);
            }
            
            if (timestamp) {
                const now = new Date();
                timestamp.textContent = `Updated at ${now.toLocaleTimeString()}`;
            }
        },
        
        renderLabelMetricsTable() {
            const tableBody = document.getElementById('labelsTableBody');
            if (tableBody) {
                tableBody.innerHTML = state.labelMetrics.map(metric => `
                    <tr>
                        <td class="table-name-cell">${metric.labels}</td>
                        <td class="table-stat-value">${metric.running_count}</td>
                        <td class="table-stat-value">${metric.queued_count}</td>
                        <td class="table-stat-value">${metric.total_count}</td>
                        <td class="table-stat-value">${metric.completed_count}</td>
                        <td>
                            <div class="table-type-badge ${metric.runner_type}">
                                <span class="table-type-icon ${metric.runner_type}"></span>
                                ${metric.runner_type === 'github-hosted' ? 'GitHub-Hosted' : 
                                  metric.runner_type === 'self-hosted' ? 'Self-Hosted' : 'Unknown'}
                            </div>
                        </td>
                    </tr>
                `).join('');
            }
        }
    };

    window.addEventListener('beforeunload', () => {
        if (workflowManager.durationUpdateInterval) {
            workflowManager.stopDurationCounter();
        }
    });

    window.workflowManager = workflowManager;
    window.labelManager = labelManager;
    initializeDashboard();
});
