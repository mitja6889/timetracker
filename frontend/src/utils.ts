import type { Entry } from './api'

const pad = (n: number) => String(n).padStart(2, '0')

export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return `${pad(h)}:${pad(m)}:${pad(sec)}`
}

export function formatDateTime(iso: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('sl-SI', { dateStyle: 'short', timeStyle: 'short' })
}

export function formatTime(iso: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleTimeString('sl-SI', { hour: '2-digit', minute: '2-digit' })
}

/** Lokalni YYYY-MM-DD iz ISO datuma (za grupiranje vnosov). */
export function localDayKey(iso: string): string {
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const dayKeyToDate = (key: string) => new Date(`${key}T12:00:00`)

/** Naslov skupine vnosov za dan (Danes, Včeraj, …). */
export function formatDayGroupLabel(dayKey: string, todayKey = localDayKey(new Date().toISOString())): string {
  if (dayKey === todayKey) return 'Danes'
  const today = dayKeyToDate(todayKey)
  const y = new Date(today)
  y.setDate(y.getDate() - 1)
  const yesterdayKey = localDayKey(y.toISOString())
  if (dayKey === yesterdayKey) return 'Včeraj'
  return dayKeyToDate(dayKey).toLocaleDateString('sl-SI', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: dayKey.slice(0, 4) !== todayKey.slice(0, 4) ? 'numeric' : undefined,
  })
}

/** ISO (UTC) -> vrednost za <input type="datetime-local"> v lokalnem času */
export function toLocalInput(iso: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** vrednost iz <input type="datetime-local"> (lokalni čas) -> ISO UTC */
export function fromLocalInput(v: string): string {
  return new Date(v).toISOString()
}

/** začetek lokalnega dneva (YYYY-MM-DD) kot ISO UTC */
export function dayStartISO(dateStr: string): string {
  return new Date(`${dateStr}T00:00:00`).toISOString()
}

/** začetek NASLEDNJEGA lokalnega dneva kot ISO UTC (zgornja meja je izključena) */
export function nextDayStartISO(dateStr: string): string {
  const d = new Date(`${dateStr}T00:00:00`)
  d.setDate(d.getDate() + 1)
  return d.toISOString()
}

/** Enak izračun kot na strežniku: čas brez pavz; tekoči vnosi štejejo do nowMs. */
export function calcWorkSeconds(e: Entry, nowMs: number): number {
  const start = new Date(e.startedAt).getTime()
  const end = e.endedAt ? new Date(e.endedAt).getTime() : nowMs
  let total = end - start
  for (const p of e.pauses) {
    let pe = p.end ? new Date(p.end).getTime() : end
    if (pe > end) pe = end
    const ps = new Date(p.start).getTime()
    if (pe > ps) total -= pe - ps
  }
  return Math.max(0, Math.floor(total / 1000))
}
