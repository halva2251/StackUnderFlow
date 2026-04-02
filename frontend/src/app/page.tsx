'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { listQuestions } from '@/lib/api'
import { Question, SortOption } from '@/types'

export default function HomePage() {
  const [questions, setQuestions] = useState<Question[]>([])
  const [sort, setSort] = useState<SortOption>('newest')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const handleSortChange = (s: SortOption) => {
    setSort(s)
    setLoading(true)
  }

  useEffect(() => {
    listQuestions({ sort, limit: 20 })
      .then(data => {
        setQuestions(data.questions)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [sort])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Questions</h1>
        <div className="flex gap-1 bg-gray-100 rounded-lg p-1 text-sm">
          {(['newest', 'most_upvoted'] as SortOption[]).map(s => (
            <button
              key={s}
              onClick={() => handleSortChange(s)}
              className={`px-3 py-1 rounded-md transition-colors ${
                sort === s ? 'bg-white shadow text-orange-600 font-medium' : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              {s === 'newest' ? 'Newest' : 'Top'}
            </button>
          ))}
        </div>
      </div>

      {loading && <p className="text-gray-500">Loading...</p>}
      {error && <p className="text-red-500">Error: {error}</p>}

      {!loading && !error && (
        <div className="space-y-4">
          {questions.length === 0 ? (
            <p className="text-gray-500">No questions yet. Be the first to ask!</p>
          ) : (
            questions.map(q => (
              <QuestionCard key={q.id} question={q} />
            ))
          )}
        </div>
      )}
    </div>
  )
}

function QuestionCard({ question }: { question: Question }) {
  const score = question.upvotes - question.downvotes

  return (
    <Link
      href={`/questions/${question.id}`}
      className="block bg-white border rounded-lg p-4 hover:border-orange-300 transition-colors"
    >
      <h2 className="text-lg font-medium text-gray-900 mb-1">{question.title}</h2>
      <p className="text-gray-600 text-sm mb-3 line-clamp-2">{question.body}</p>
      <div className="flex items-center gap-4 text-xs text-gray-500">
        <span className={score >= 0 ? 'text-green-600' : 'text-red-600'}>
          {score >= 0 ? '+' : ''}{score} votes
        </span>
        <span>{question.upvotes} upvotes</span>
        <span>{question.downvotes} downvotes</span>
        <span>{new Date(question.created_at).toLocaleDateString()}</span>
      </div>
    </Link>
  )
}
