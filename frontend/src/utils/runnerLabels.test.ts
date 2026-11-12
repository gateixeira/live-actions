import { describe, expect, it } from 'vitest'
import { runnerLabel, sharedRunnerPrefix } from './runnerLabels'

describe('sharedRunnerPrefix', () => {
  it('returns the shared prefix up to the last separator', () => {
    expect(sharedRunnerPrefix(['acme-runner-3fe3e3-d1f9', 'acme-runner-3a0b12-77aa', 'acme-runner-b9e0c8-b903'])).toBe('acme-runner-')
  })

  it('never cuts inside a word', () => {
    expect(sharedRunnerPrefix(['build-alpha', 'build-alps'])).toBe('build-')
    expect(sharedRunnerPrefix(['alpha', 'alps'])).toBe('')
  })

  it('is empty for a single runner or none', () => {
    expect(sharedRunnerPrefix(['acme-runner-1'])).toBe('')
    expect(sharedRunnerPrefix([])).toBe('')
  })
})

describe('runnerLabel', () => {
  it('drops the shared prefix', () => {
    expect(runnerLabel('acme-runner-3fe3e3-d1f9', 'acme-runner-', 22)).toBe('3fe3e3-d1f9')
  })

  it('keeps the whole name when the prefix is the whole name or absent', () => {
    expect(runnerLabel('acme-runner-', 'acme-runner-', 22)).toBe('acme-runner-')
    expect(runnerLabel('GitHub Actions 12', 'acme-runner-', 22)).toBe('GitHub Actions 12')
  })

  it('shortens long names in the middle so both ends stay visible', () => {
    const label = runnerLabel('a-very-long-runner-name-that-ends-in-42', '', 15)
    expect(label).toHaveLength(15)
    expect(label.startsWith('a-very-')).toBe(true)
    expect(label.endsWith('-in-42')).toBe(true)
    expect(label).toContain('…')
  })
})
