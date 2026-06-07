'use client'

export type ApiBody<T> = {
  code: number
  message: string
  data: T
  request_id?: string
}

export type User = {
  id: number
  email: string
  username: string
  role: string
  status: string
}

export type Conversation = {
  id: number
  user_id: number
  title: string
  created_at: string
  updated_at: string
}

export type Message = {
  id: number
  conversation_id: number
  user_id: number
  role: 'user' | 'assistant'
  content: string
  created_at: string
}

export type FileRecord = {
  id: number
  original_name: string
  size: number
  status: string
  error?: string
  indexed_count: number
  created_at: string
}

export type AdminStats = {
  users: number
  conversations: number
  messages: number
  files: number
  model_calls: number
  indexed_files: number
  failed_files: number
}

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080'
const TOKEN_KEY = 'qqqai_token'

export function getToken() {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })
  const body = (await res.json()) as ApiBody<T>
  if (!res.ok || body.code !== 0) {
    if (res.status === 401) {
      clearToken()
      window.location.href = '/login'
    }
    throw new Error(body.message || `HTTP ${res.status}`)
  }
  return body.data
}

export async function streamChat(
  conversationId: number,
  message: string,
  onChunk: (text: string) => void,
) {
  const res = await fetch(`${API_BASE}/api/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
    },
    body: JSON.stringify({ conversation_id: conversationId || 0, message }),
  })
  if (!res.ok || !res.body) {
    throw new Error(`stream failed: HTTP ${res.status}`)
  }
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const events = buffer.split('\n\n')
    buffer = events.pop() || ''
    for (const event of events) {
      const line = event.split('\n').find((item) => item.startsWith('data: '))
      if (!line) continue
      const payload = JSON.parse(line.slice(6)) as { content?: string; error?: string; done?: boolean }
      if (payload.error) throw new Error(payload.error)
      if (payload.content) onChunk(payload.content)
    }
  }
}
