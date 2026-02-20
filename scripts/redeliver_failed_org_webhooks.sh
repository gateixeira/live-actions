#!/usr/bin/env bash
#
# Redeliver failed organization webhook deliveries from the last hour.
#
# This script queries the GitHub API for webhook deliveries that failed
# (non-2xx status code or status != "OK") within the past hour and
# triggers a redelivery attempt for each one. Deliveries are grouped by
# event guid — if any retry for an event already succeeded, the earlier
# failure is skipped to avoid duplicate redeliveries.
#
# Prerequisites:
#   - gh CLI installed and authenticated with admin:org_hook scope
#
# Usage:
#   ./redeliver_failed_org_webhooks.sh <hook-id> <organization>
#
# Arguments:
#   hook-id       The numeric ID of the organization webhook
#   organization  The GitHub organization slug (e.g. "my-org")
#
# Examples:
#   ./redeliver_failed_org_webhooks.sh 123456 my-org
#   ./redeliver_failed_org_webhooks.sh 789012 acme-corp

set -euo pipefail

log() {
  echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] $*"
}

usage() {
  echo "Usage: $0 <hook-id> <organization>"
  echo "Example: $0 123456 my-org"
}

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

hook_id="$1"
org="$2"

log "Starting redelivery check for hook ${hook_id} in org ${org}"

if ! command -v gh >/dev/null 2>&1; then
  log "ERROR: gh CLI is required but not found in PATH."
  exit 1
fi

log "Computing cutoff timestamp (1 hour ago)..."
if cutoff_epoch="$(date -u -v-1H +%s 2>/dev/null)"; then
  :
else
  cutoff_epoch="$(date -u -d "1 hour ago" +%s)"
fi
log "Cutoff epoch: ${cutoff_epoch} ($(date -u -r "${cutoff_epoch}" '+%Y-%m-%d %H:%M:%S UTC' 2>/dev/null || date -u -d "@${cutoff_epoch}" '+%Y-%m-%d %H:%M:%S UTC' 2>/dev/null || echo 'N/A'))"

log "Fetching deliveries from GitHub API..."
# Fetch all recent deliveries (both failed and successful) as JSON lines.
# Each line contains: id, guid, status_code, status, delivered_at epoch.
all_deliveries="$(gh api --paginate "/orgs/${org}/hooks/${hook_id}/deliveries?per_page=100" \
  | jq -rc --argjson cutoff "${cutoff_epoch}" '.[] | select((.delivered_at | sub("\\.[0-9]+"; "") | fromdateiso8601) >= $cutoff) | {id: (.id | tostring), guid, status_code: (.status_code // 0), status: ((.status // "") | ascii_upcase), delivered_at: (.delivered_at | sub("\\.[0-9]+"; "") | fromdateiso8601)}')"

if [[ -z "${all_deliveries}" ]]; then
  log "No deliveries found in the last hour for hook ${hook_id} in org ${org}."
  exit 0
fi

# Identify failed delivery IDs, excluding any whose guid had a later successful attempt.
# A delivery is "successful" if status_code is 2xx AND status is "OK".
# jq groups by guid: if any attempt for that guid succeeded, skip all failures for it.
delivery_ids="$(echo "${all_deliveries}" | jq -r -s '
  group_by(.guid) |
  map(
    select(all(
      (.status_code < 200 or .status_code >= 300) or (.status != "OK")
    )) |
    sort_by(.delivered_at) |
    last | .id
  ) | .[]
')"

if [[ -z "${delivery_ids}" ]]; then
  log "No failed deliveries without a successful retry found in the last hour for hook ${hook_id} in org ${org}."
  exit 0
fi

total="$(echo "${delivery_ids}" | grep -c .)"
log "Found ${total} failed delivery(s) to redeliver."

count=0
success=0
while IFS= read -r delivery_id; do
  [[ -z "${delivery_id}" ]] && continue
  count=$((count + 1))
  log "Redelivering ${count}/${total}: delivery ${delivery_id}..."
  if gh api -X POST "/orgs/${org}/hooks/${hook_id}/deliveries/${delivery_id}/attempts" >/dev/null; then
    success=$((success + 1))
    log "  ✓ Delivery ${delivery_id} redelivered successfully."
  else
    log "  ✗ ERROR: Failed to redeliver delivery ${delivery_id} (exit code: $?)."
  fi
done <<< "${delivery_ids}"

log "Completed. Successfully redelivered ${success}/${total} failed delivery(s)."
