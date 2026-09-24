export const formatDate = (value: string | null | undefined) => value
  ? new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(value))
  : '—'

export const fixed = (value: number | null | undefined, places = 1) => Number.isFinite(value) ? Number(value).toFixed(places) : '—'

export const shortHash = (value: string | null | undefined) => value ? `${value.slice(0, 10)}…${value.slice(-6)}` : '—'

export const signedFixed = (value: number | null | undefined, places = 1, suffix = '') => {
  if (!Number.isFinite(value)) return '—'
  const number = Number(value)
  if (Object.is(number, -0)) return `0.${'0'.repeat(places)}${suffix}`
  const sign = number > 0 ? '+' : number < 0 ? '−' : ''
  return `${sign}${Math.abs(number).toFixed(places)}${suffix}`
}
