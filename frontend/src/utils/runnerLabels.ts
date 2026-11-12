// Chart labels for runner names. Fleet runners usually share a long prefix
// (e.g. "acme-runner-"), so a plain cut leaves every label identical; drop the
// shared prefix and shorten the middle so the distinctive end stays visible.

/** The prefix all names share, cut back to its last '-', '_' or '.'; '' for fewer than 2 names. */
export function sharedRunnerPrefix(names: string[]): string {
  if (names.length < 2) return ''
  let prefix = names[0]
  for (const name of names) {
    while (prefix && !name.startsWith(prefix)) prefix = prefix.slice(0, -1)
  }
  const cut = Math.max(prefix.lastIndexOf('-'), prefix.lastIndexOf('_'), prefix.lastIndexOf('.'))
  return cut >= 0 ? prefix.slice(0, cut + 1) : ''
}

/** Name without the shared prefix, then shortened in the middle to at most maxLen characters. */
export function runnerLabel(name: string, prefix: string, maxLen: number): string {
  const short = prefix && name.startsWith(prefix) && name.length > prefix.length ? name.slice(prefix.length) : name
  if (short.length <= maxLen) return short
  const keep = maxLen - 1
  const head = Math.ceil(keep / 2)
  return short.slice(0, head) + '…' + short.slice(short.length - (keep - head))
}
