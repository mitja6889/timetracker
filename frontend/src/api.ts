export interface Me {
  id: string
  username: string
  name: string
  roleId: string
  roleName: string
  permissions: string[]
}

export interface Pause {
  start: string
  end: string | null
}

export type TimeEventType = 'start' | 'stop'

export interface TimeEvent {
  type: TimeEventType
  at: string
}

export type EntryStatus = 'running' | 'paused' | 'stopped'

export interface Entry {
  id: string
  userId: string
  username: string
  name: string
  description: string
  startedAt: string
  endedAt: string | null
  events: TimeEvent[]
  pauses?: Pause[]
  status: EntryStatus
  workSeconds: number
  canEdit: boolean
  canDelete: boolean
  createdAt: string
  updatedAt: string
}

export interface EntriesPage {
  items: Entry[]
  total: number
  page: number
  pageSize: number
  /** Vsota delovnega časa vseh vnosov, ki ustrezajo filtrom (vključno z obdobjem). */
  rangeWorkSeconds?: number
}

export interface TimerState {
  running: boolean
  runningSince: string | null
  sessionSeconds: number
  now: string
}

export interface TimeEventRow {
  id: string
  type: TimeEventType
  at: string
}

export interface DayPauseRow {
  id: string
  startAt: string
  endAt: string
  note?: string
}

export interface TimeDay {
  dayKey: string
  events: TimeEventRow[]
  pauses: DayPauseRow[]
  workSeconds: number
}

export interface TimeDaysResult {
  days: TimeDay[]
  rangeWorkSeconds: number
}

export interface Role {
  id: string
  name: string
  description: string
  permissions: string[]
  system: boolean
  userCount: number
}

export interface UserRow {
  id: string
  username: string
  name: string
  roleId?: string
  roleName?: string
  active?: boolean
  createdAt?: string
}

export interface PermissionInfo {
  key: string
  label: string
}

export interface EntryQuery {
  page: number
  pageSize: number
  sort: string
  order: 'asc' | 'desc'
  q?: string
  status?: string
  userId?: string
  from?: string
  to?: string
}

export interface EntryPayload {
  userId?: string
  description: string
  startedAt: string
  endedAt: string
  pauses: { start: string; end: string }[]
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

const TOKEN_KEY = 'tt_token'

export const tokenStore = {
  get: (): string | null => {
    try {
      return localStorage.getItem(TOKEN_KEY)
    } catch {
      return null
    }
  },
  set: (t: string) => {
    try {
      localStorage.setItem(TOKEN_KEY, t)
    } catch {
      /* ignore */
    }
  },
  clear: () => {
    try {
      localStorage.removeItem(TOKEN_KEY)
    } catch {
      /* ignore */
    }
  },
}

let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

async function request<T>(method: string, path: string, body?: unknown, auth = true): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = tokenStore.get()
  if (auth && token) headers['Authorization'] = `Bearer ${token}`

  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError(0, 'Strežnik ni dosegljiv')
  }

  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }

  if (!res.ok) {
    if (res.status === 401 && auth) onUnauthorized()
    const msg =
      data && typeof data === 'object' && 'error' in data
        ? String((data as { error: unknown }).error)
        : `Napaka ${res.status}`
    throw new ApiError(res.status, msg)
  }
  return data as T
}

function qs(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}

export const api = {
  login: (username: string, password: string) =>
    request<{ token: string; me: Me }>('POST', '/api/auth/login', { username, password }, false),
  me: () => request<Me>('GET', '/api/auth/me'),

  permissions: () => request<PermissionInfo[]>('GET', '/api/permissions'),

  roles: () => request<Role[]>('GET', '/api/roles'),
  createRole: (r: { name: string; description: string; permissions: string[] }) =>
    request<Role>('POST', '/api/roles', r),
  updateRole: (id: string, r: { name: string; description: string; permissions: string[] }) =>
    request<{ status: string }>('PUT', `/api/roles/${id}`, r),
  deleteRole: (id: string) => request<{ status: string }>('DELETE', `/api/roles/${id}`),

  users: () => request<UserRow[]>('GET', '/api/users'),
  createUser: (u: {
    username: string
    name: string
    password: string
    roleId: string
    active: boolean
  }) => request<{ id: string }>('POST', '/api/users', u),
  updateUser: (
    id: string,
    u: { name: string; roleId: string; active: boolean; password?: string },
  ) => request<{ status: string }>('PUT', `/api/users/${id}`, u),
  deleteUser: (id: string) => request<{ status: string }>('DELETE', `/api/users/${id}`),

  timerCurrent: () => request<TimerState>('GET', '/api/timer/current'),
  timerStart: () => request<TimerState>('POST', '/api/timer/start'),
  timerStop: () => request<TimerState>('POST', '/api/timer/stop'),

  timeDays: (q: { from: string; to: string; userId?: string }) =>
    request<TimeDaysResult>(
      'GET',
      '/api/time/days' + qs({ from: q.from, to: q.to, userId: q.userId }),
    ),
  createDayPause: (
    dayKey: string,
    body: { startAt: string; endAt: string; note?: string; userId?: string },
  ) => request<{ id: string }>('POST', `/api/time/days/${dayKey}/pauses`, body),
  deleteDayPause: (id: string) => request<{ status: string }>('DELETE', `/api/time/pauses/${id}`),

  entries: (q: EntryQuery) =>
    request<EntriesPage>(
      'GET',
      '/api/entries' +
        qs({
          page: q.page,
          pageSize: q.pageSize,
          sort: q.sort,
          order: q.order,
          q: q.q,
          status: q.status,
          userId: q.userId,
          from: q.from,
          to: q.to,
        }),
    ),
  createEntry: (e: EntryPayload) => request<{ id: string }>('POST', '/api/entries', e),
  updateEntry: (id: string, e: EntryPayload) =>
    request<{ status: string }>('PUT', `/api/entries/${id}`, e),
  deleteEntry: (id: string) => request<{ status: string }>('DELETE', `/api/entries/${id}`),
}
