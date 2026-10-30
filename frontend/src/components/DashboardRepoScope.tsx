type DashboardRepoScopeProps = {
  repo: string
}

export function DashboardRepoScope({ repo }: DashboardRepoScopeProps) {
  if (!repo) return null

  return (
    <p role="note" className="rounded-lg border border-gray-700 bg-gray-900/60 px-4 py-2 text-xs text-gray-400">
      Workflow runs and analytics are filtered to {repo}. Summary cards and Runner Demand show all repositories.
    </p>
  )
}
