'use client'

import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { User } from '@/types'

interface AuthContextValue {
  user: User | null
  token: string | null
  login: (token: string, user: User) => void
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const stored = localStorage.getItem('auth')
    if (stored) {
      try {
        const { token: t, user: u } = JSON.parse(stored)
        setToken(t)
        setUser(u)
      } catch {
        localStorage.removeItem('auth')
      }
    }
    setIsLoading(false)
  }, [])

  const login = (token: string, user: User) => {
    setToken(token)
    setUser(user)
    localStorage.setItem('auth', JSON.stringify({ token, user }))
  }

  const logout = () => {
    setToken(null)
    setUser(null)
    localStorage.removeItem('auth')
  }

  return (
    <AuthContext.Provider value={{ user, token, login, logout, isLoading }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
