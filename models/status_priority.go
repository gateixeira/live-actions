package models

// Status priorities express the monotonic lifecycle ordering of webhook
// events. They are used both:
//   - by the ingest pipeline when sorting events delivered out-of-order from
//     GitHub, and
//   - by the typed-table UPSERTs as a guard against an older event
//     overwriting a newer one (see AddOrUpdateJob / AddOrUpdateRun).
//
// Centralising these mappings here is what makes the typed-table writes
// order-independent: handlers no longer need to rely on a polling-based
// ordering window to deliver events in lifecycle order.

// JobStatusPriority returns the lifecycle priority of a workflow job status.
// The boolean is false for unknown / unsupported statuses; callers should
// treat that as a permanent rejection rather than substituting a default
// (a high default would let unknown statuses overwrite valid terminal rows;
// a low default would silently drop legitimate progressions).
func JobStatusPriority(status JobStatus) (int, bool) {
	switch status {
	case JobStatusWaiting:
		return 1, true
	case JobStatusQueued:
		return 2, true
	case JobStatusRequested:
		return 3, true
	case JobStatusInProgress:
		return 4, true
	case JobStatusCompleted, JobStatusCancelled, JobStatusStale:
		return 5, true
	}
	return 0, false
}

// RunStatusPriority returns the lifecycle priority of a workflow run status.
// Runs do not emit waiting/queued events, so the mapping is shorter than the
// job mapping. See JobStatusPriority for the unknown-status rationale.
func RunStatusPriority(status JobStatus) (int, bool) {
	switch status {
	case JobStatusRequested:
		return 1, true
	case JobStatusInProgress:
		return 2, true
	case JobStatusCompleted, JobStatusCancelled:
		return 3, true
	}
	return 0, false
}

// IsTerminalJobStatus reports whether the given status is a final state for
// a workflow job. Once a job reaches a terminal state, only same-status
// replays are accepted; a different terminal (e.g. completed → cancelled)
// is rejected to avoid corrupting the historical record.
func IsTerminalJobStatus(status JobStatus) bool {
	switch status {
	case JobStatusCompleted, JobStatusCancelled, JobStatusStale:
		return true
	}
	return false
}

// IsTerminalRunStatus reports whether the given status is a final state for
// a workflow run.
func IsTerminalRunStatus(status JobStatus) bool {
	switch status {
	case JobStatusCompleted, JobStatusCancelled:
		return true
	}
	return false
}
