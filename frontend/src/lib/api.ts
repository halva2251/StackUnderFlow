const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080/api/v1'

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const stored = typeof window !== 'undefined' ? localStorage.getItem('auth') : null
  const token = stored ? JSON.parse(stored)?.token : null

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: { message: res.statusText } }))
    throw new Error(body.error?.message ?? `Request failed: ${res.status}`)
  }

  return res.json()
}

// Auth
export const register = (username: string, password: string) =>
  request<{ token: string; user: { id: string; username: string } }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })

export const login = (username: string, password: string) =>
  request<{ token: string; user: { id: string; username: string } }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })

// Questions
export const listQuestions = (params?: { sort?: string; limit?: number; offset?: number }) => {
  const qs = new URLSearchParams()
  if (params?.sort) qs.set('sort', params.sort)
  if (params?.limit) qs.set('limit', String(params.limit))
  if (params?.offset) qs.set('offset', String(params.offset))
  const query = qs.toString()
  return request<{ questions: import('@/types').Question[]; total: number; limit: number; offset: number; sort: string }>(
    `/questions${query ? `?${query}` : ''}`
  )
}

export const getQuestion = (id: string) =>
  request<{ question: import('@/types').Question; answers: import('@/types').Answer[] }>(`/questions/${id}`)

export const createQuestion = (title: string, body: string) =>
  request<{ question: import('@/types').Question }>('/questions', {
    method: 'POST',
    body: JSON.stringify({ title, body }),
  })

export const argueQuestion = (id: string, argument: string) =>
  request<{ question: import('@/types').Question; answers: import('@/types').Answer[] }>(
    `/questions/${id}/argue`,
    { method: 'POST', body: JSON.stringify({ argument }) }
  )

// Votes
export const voteQuestion = (id: string, value: 1 | -1) =>
  request<{ upvotes: number; downvotes: number }>(`/questions/${id}/vote`, {
    method: 'PUT',
    body: JSON.stringify({ value }),
  })

export const removeQuestionVote = (id: string) =>
  request<{ upvotes: number; downvotes: number }>(`/questions/${id}/vote`, {
    method: 'DELETE',
  })

export const voteAnswer = (id: string, value: 1 | -1) =>
  request<{ upvotes: number; downvotes: number }>(`/answers/${id}/vote`, {
    method: 'PUT',
    body: JSON.stringify({ value }),
  })

export const removeAnswerVote = (id: string) =>
  request<{ upvotes: number; downvotes: number }>(`/answers/${id}/vote`, {
    method: 'DELETE',
  })
