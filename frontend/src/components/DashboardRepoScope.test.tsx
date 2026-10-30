import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DashboardRepoScope } from './DashboardRepoScope'

describe('DashboardRepoScope', () => {
  it('explains which dashboard sections remain global when a repository is selected', () => {
    render(<DashboardRepoScope repo="owner/project" />)

    expect(screen.getByRole('note')).toHaveTextContent(
      'Workflow runs and analytics are filtered to owner/project. Summary cards and Runner Demand show all repositories.',
    )
  })

  it('does not show an extra scope note when all repositories are selected', () => {
    render(<DashboardRepoScope repo="" />)

    expect(screen.queryByRole('note')).not.toBeInTheDocument()
  })
})
