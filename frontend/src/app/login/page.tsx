'use client'

import { useState } from 'react'
import { login } from '@/lib/api'
import { useAuth } from '@/context/AuthContext'
import { useRouter } from 'next/navigation'
import Link from 'next/link'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const { login: setAuth } = useAuth()
  const router = useRouter()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await login(username, password)
      setAuth(data.token, data.user)
      router.push('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-sm mx-auto mt-10">
      <h1 className="text-2xl font-bold mb-6">Login</h1>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <p className="text-red-500 text-sm">{error}</p>}
        <div>
          <label className="block text-sm font-medium mb-1">Username</label>
          <input
            type="text"
            value={username}
            onChange={e => setUsername(e.target.value)}
            className="w-full border rounded-md px-3 py-2"
            required
          />
        </div>
        <div>
          <label className="block text-sm font-medium mb-1">Password</label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            className="w-full border rounded-md px-3 py-2"
            required
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="w-full bg-orange-500 text-white py-2 rounded-md font-medium hover:bg-orange-600 disabled:opacity-50 transition-colors"
        >
          {loading ? 'Logging in...' : 'Login'}
        </button>
      </form>
      <p className="mt-4 text-center text-sm text-gray-600">
        Don&apos;t have an account? <Link href="/register" className="text-orange-600 hover:underline">Register</Link>
      </p>
    </div>
  )
}
