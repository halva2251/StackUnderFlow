'use client'

import { useState } from 'react'
import { createQuestion } from '@/lib/api'
import { useAuth } from '@/context/AuthContext'
import { useRouter } from 'next/navigation'

export default function AskPage() {
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const { user } = useAuth()
  const router = useRouter()

  if (!user) {
    router.push('/login')
    return null
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await createQuestion(title, body)
      router.push(`/questions/${data.question.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post question')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Ask a Question</h1>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <p className="text-red-500 text-sm">{error}</p>}
        <div>
          <label className="block text-sm font-medium mb-1">Title</label>
          <input
            type="text"
            value={title}
            onChange={e => setTitle(e.target.value)}
            placeholder="What's your question?"
            className="w-full border rounded-md px-3 py-2"
            required
          />
        </div>
        <div>
          <label className="block text-sm font-medium mb-1">Details</label>
          <textarea
            value={body}
            onChange={e => setBody(e.target.value)}
            placeholder="Provide more context..."
            rows={8}
            className="w-full border rounded-md px-3 py-2 resize-none"
            required
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="bg-orange-500 text-white px-6 py-2 rounded-md font-medium hover:bg-orange-600 disabled:opacity-50 transition-colors"
        >
          {loading ? 'Posting...' : 'Post Question'}
        </button>
      </form>
    </div>
  )
}
