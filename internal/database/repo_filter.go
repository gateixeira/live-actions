package database

// jobRepoFilter returns a JOIN clause and args for filtering workflow_jobs by repository.
// When repo is empty, returns empty string and nil args (no filter).
func jobRepoFilter(repo string) (string, []interface{}) {
	if repo == "" {
		return "", nil
	}
	return " JOIN workflow_runs r ON j.run_id = r.id", []interface{}{repo}
}

// repoWhere returns the AND clause for repo filtering.
func repoWhere(repo string) string {
	if repo == "" {
		return ""
	}
	return " AND r.repository = ?"
}
