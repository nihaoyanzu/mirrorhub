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

/** 将毫秒格式化为可读时长（秒 / 分钟 / 小时 / 天） */
export function fmtDurationMs(ms: number | null | undefined, locale?: string): string {
  if (ms == null || Number.isNaN(Number(ms)) || ms < 0) return '-'
  const n = Math.floor(Number(ms))
  const zh = !locale || String(locale).toLowerCase().startsWith('zh')
  const fmt = (v: number, zhUnit: string, enUnit: string) => {
    const text = String(Math.max(1, Math.round(v)))
    return zh ? `${text} ${zhUnit}` : `${text} ${enUnit}`
  }
  if (n < 1000) return zh ? `${n} 毫秒` : `${n} ms`
  const sec = n / 1000
  if (sec < 60) return fmt(sec, '秒', 's')
  const min = sec / 60
  if (min < 60) return fmt(min, '分钟', 'min')
  const hr = min / 60
  if (hr < 48) return fmt(hr, '小时', 'h')
  return fmt(hr / 24, '天', 'd')
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
