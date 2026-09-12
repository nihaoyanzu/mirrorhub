export function fmtBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${u[i]}`
}

export function fmtRate(bps: number): string {
  if (!bps || bps < 1) return '0 B/s'
  return `${fmtBytes(bps)}/s`
}

export function fmtTime(v: string | number | Date | null | undefined): string {
  if (v == null || v === '') return '-'
  // Go RFC3339Nano 小数位常超过 3 位，部分环境 Date 解析会失败
  const raw = typeof v === 'string' ? v.replace(/(\.\d{3})\d+/, '$1') : v
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

export function hitRate(hits: number, misses: number): string {
  const total = hits + misses
  if (!total) return '-'
  return `${((hits / total) * 100).toFixed(1)}%`
}
