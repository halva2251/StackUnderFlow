export interface User {
  id: string
  username: string
  created_at?: string
}

export interface Question {
  id: string
  user_id: string
  title: string
  body: string
  upvotes: number
  downvotes: number
  created_at: string
}

export interface Answer {
  id: string
  question_id: string
  user_id: string
  depth: number
  user_prompt: string
  ai_response: string
  upvotes: number
  downvotes: number
  created_at: string
}

export interface QuestionListResponse {
  questions: Question[]
  total: number
  limit: number
  offset: number
  sort: string
}

export interface QuestionResponse {
  question: Question
  answers: Answer[]
}

export interface AuthResponse {
  token: string
  user: User
}

export interface ApiError {
  error: {
    code: string
    message: string
  }
}

export type SortOption = 'newest' | 'most_upvoted'
