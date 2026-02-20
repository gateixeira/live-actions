import { useTheme } from '@primer/react'

/**
 * Returns chart-friendly color values that match the active Primer color mode.
 * Recharts needs resolved hex strings — it can't consume CSS variables or sx tokens.
 * Values sourced from @primer/primitives light/dark palettes.
 */
export function useChartColors() {
  const { resolvedColorMode } = useTheme()
  const isDark = resolvedColorMode === 'night'

  return {
    success: isDark ? '#3fb950' : '#1a7f37',
    danger: isDark ? '#f85149' : '#cf222e',
    attention: isDark ? '#d29922' : '#9a6700',
    accent: isDark ? '#58a6ff' : '#0969da',
    done: isDark ? '#bc8cff' : '#8250df',
    sponsors: isDark ? '#db61a2' : '#bf3989',
    muted: isDark ? '#8b949e' : '#656d76',
    // Multi-series palette for charts with many data series
    palette: isDark
      ? ['#3fb950', '#58a6ff', '#d29922', '#f85149', '#bc8cff', '#39d353', '#79c0ff', '#e3b341', '#ffa198', '#d2a8ff']
      : ['#1a7f37', '#0969da', '#9a6700', '#cf222e', '#8250df', '#2da44e', '#0550ae', '#bf8700', '#a40e26', '#6639ba'],
  }
}
