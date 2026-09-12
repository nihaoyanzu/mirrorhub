/** 任务 / 缓存状态 — 返回 i18n key，由组件侧 t() 翻译 */

export function statusKey(s: string): string {
  switch (s) {
    case 'running': return 'status.running'
    case 'done': return 'status.done'
    case 'error': return 'status.error'
    case 'cancelled': return 'status.cancelled'
    case 'queued': return 'status.queued'
    default: return s || '-'
  }
}

export function statusClass(s: string): string {
  if (s === 'running') return 'ui-badge-ok'
  if (s === 'error') return 'ui-badge-danger'
  if (s === 'cancelled') return 'ui-badge-warn'
  if (s === 'queued') return 'ui-badge'
  return 'ui-badge-muted'
}

export function prioClass(p: string): string {
  if (p === 'P0' || p === 'P0+') return 'ui-badge-warn'
  if (p === 'P1') return 'ui-badge-ok'
  if (p === 'P2') return 'ui-badge-muted'
  return 'ui-badge'
}

export function prioKey(p: string): string {
  if (p === 'P0+') return 'status.p0Boost'
  if (p === 'P0') return 'status.p0Interactive'
  if (p === 'P1') return 'status.p1Resume'
  if (p === 'P2') return 'status.p2Prefetch'
  return p || '-'
}

export function cacheKey(c: string): string {
  if (c === 'hit') return 'status.hit'
  if (c === 'miss') return 'status.miss'
  if (c === 'na') return '-'
  return c || '-'
}

export function cacheClass(c: string): string {
  if (c === 'hit') return 'ui-badge-ok'
  if (c === 'miss') return 'ui-badge-warn'
  return 'ui-badge-muted'
}
