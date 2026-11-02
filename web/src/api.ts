let csrfToken = ''

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

export function setCSRF(token: string) {
  csrfToken = token
}

interface Envelope<T> {
  code: number
  message: string
  data: T
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const method = (options.method ?? 'GET').toUpperCase()
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (csrfToken && method !== 'GET' && method !== 'HEAD') {
    headers.set('X-CSRF-Token', csrfToken)
  }

  const response = await fetch(`/api/v1${path}`, {
    ...options,
    method,
    headers,
    credentials: 'same-origin',
  })

  let payload: Envelope<T> | null = null
  try {
    payload = (await response.json()) as Envelope<T>
  } catch {
    payload = null
  }

  if (!response.ok) {
    throw new ApiError(payload?.message || `请求失败 (${response.status})`, response.status)
  }
  return (payload?.data ?? (payload as unknown)) as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  upload: async (file: File): Promise<{ url: string; key: string; size: number }> => {
    const form = new FormData()
    form.append('file', file)
    return request('/uploads', { method: 'POST', body: form })
  },
}
