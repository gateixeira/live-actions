#!/usr/bin/env bash

set -euo pipefail

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

if ! command -v gh >/dev/null 2>&1; then
  echo "Error: gh CLI is required."
  exit 1
fi

if cutoff_epoch="$(date -u -v-1H +%s 2>/dev/null)"; then
  :
else
  cutoff_epoch="$(date -u -d "1 hour ago" +%s)"
fi

delivery_ids="$(gh api --paginate "/orgs/${org}/hooks/${hook_id}/deliveries?per_page=100" \
  --jq ".[] | select((.delivered_at | fromdateiso8601) >= ${cutoff_epoch}) | select((((.status_code // 0) < 200) or ((.status_code // 0) >= 300)) or (((.status // \"\") | ascii_upcase) != \"OK\")) | .id")"

if [[ -z "${delivery_ids}" ]]; then
  echo "No failed deliveries found in the last hour for hook ${hook_id} in org ${org}."
  exit 0
fi

count=0
while IFS= read -r delivery_id; do
  [[ -z "${delivery_id}" ]] && continue
  gh api -X POST "/orgs/${org}/hooks/${hook_id}/deliveries/${delivery_id}/attempts" >/dev/null
  echo "Redelivered delivery ${delivery_id}"
  count=$((count + 1))
done <<< "${delivery_ids}"

echo "Completed. Redelivered ${count} failed delivery(s)."
