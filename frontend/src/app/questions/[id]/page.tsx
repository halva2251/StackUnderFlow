'use client'

import { useState, useEffect, use } from 'react'
import { getQuestion, argueQuestion, voteQuestion, removeQuestionVote, voteAnswer, removeAnswerVote } from '@/lib/api'
import { Question, Answer } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { useRouter } from 'next/navigation'

export default function QuestionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const [question, setQuestion] = useState<Question | null>(null)
  const [answers, setAnswers] = useState<Answer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const { user } = useAuth()
  const router = useRouter()

  const loadQuestion = () => {
    getQuestion(id)
      .then(data => {
        setQuestion(data.question)
        setAnswers(data.answers)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(loadQuestion, [id])

  const handleVote = async (type: 'question' | 'answer', answerId: string, value: 1 | -1) => {
    if (!user) {
      router.push('/login')
      return
    }
    try {
      if (type === 'question') {
        const result = await voteQuestion(answerId, value)
        setQuestion(q => q ? { ...q, upvotes: result.upvotes, downvotes: result.downvotes } : q)
      } else {
        const result = await voteAnswer(answerId, value)
        setAnswers(as => as.map(a => a.id === answerId ? { ...a, upvotes: result.upvotes, downvotes: result.downvotes } : a))
      }
    } catch (err) {
      alert('Failed to vote. Please try again.')
    }
  }

  const handleRemoveVote = async (type: 'question' | 'answer', answerId: string) => {
    if (!user) return
    try {
      if (type === 'question') {
        const result = await removeQuestionVote(answerId)
        setQuestion(q => q ? { ...q, upvotes: result.upvotes, downvotes: result.downvotes } : q)
      } else {
        const result = await removeAnswerVote(answerId)
        setAnswers(as => as.map(a => a.id === answerId ? { ...a, upvotes: result.upvotes, downvotes: result.downvotes } : a))
      }
    } catch (err) {
      alert('Failed to remove vote. Please try again.')
    }
  }

  if (loading) return <p className="text-gray-500">Loading...</p>
  if (error) return <p className="text-red-500">Error: {error}</p>
  if (!question) return null

  return (
    <div>
      <QuestionDetail question={question} onVote={(value) => handleVote('question', question.id, value)} onRemoveVote={() => handleRemoveVote('question', question.id)} />
      <ArgueSection questionId={id} onArgued={loadQuestion} />
      <div className="mt-8">
        <h2 className="text-xl font-bold mb-4">AI Answers</h2>
        <div className="space-y-4">
          {answers.map(answer => (
            <AnswerCard key={answer.id} answer={answer} onVote={(value) => handleVote('answer', answer.id, value)} onRemoveVote={() => handleRemoveVote('answer', answer.id)} />
          ))}
        </div>
      </div>
    </div>
  )
}

function QuestionDetail({ question, onVote, onRemoveVote }: { question: Question; onVote: (v: 1 | -1) => void; onRemoveVote: () => void }) {
  const score = question.upvotes - question.downvotes

  return (
    <div className="bg-white border rounded-lg p-6 mb-6">
      <h1 className="text-2xl font-bold mb-4">{question.title}</h1>
      <p className="text-gray-700 whitespace-pre-wrap mb-4">{question.body}</p>
      <div className="flex items-center gap-4">
        <VoteButtons score={score} upvotes={question.upvotes} downvotes={question.downvotes} onVote={onVote} onRemoveVote={onRemoveVote} />
        <span className="text-xs text-gray-500">{new Date(question.created_at).toLocaleString()}</span>
      </div>
    </div>
  )
}

function VoteButtons({ score, upvotes, downvotes, onVote, onRemoveVote }: {
  score: number; upvotes: number; downvotes: number; onVote: (v: 1 | -1) => void; onRemoveVote: () => void
}) {
  return (
    <div className="flex items-center gap-2">
      <button onClick={() => onVote(1)} className="p-1 text-gray-400 hover:text-green-600" title="Upvote">▲</button>
      <span className={`font-bold ${score > 0 ? 'text-green-600' : score < 0 ? 'text-red-600' : 'text-gray-500'}`}>{score}</span>
      <button onClick={() => onVote(-1)} className="p-1 text-gray-400 hover:text-red-600" title="Downvote">▼</button>
      <button onClick={onRemoveVote} className="p-1 text-xs text-gray-300 hover:text-gray-500 ml-1" title="Clear Vote">✕</button>
      <span className="text-xs text-gray-400">{upvotes}↑ {downvotes}↓</span>
    </div>
  )
}

function AnswerCard({ answer, onVote, onRemoveVote }: { answer: Answer; onVote: (v: 1 | -1) => void; onRemoveVote: () => void }) {
  const score = answer.upvotes - answer.downvotes
  const depthColors = ['text-blue-600', 'text-purple-600', 'text-pink-600', 'text-red-600']
  const depthLabels = ['Initial Answer', 'Escalation 1', 'Escalation 2', 'Absurdity Max']

  return (
    <div className="bg-white border rounded-lg p-4">
      <div className="flex items-center gap-2 mb-3">
        <span className={`text-xs font-bold ${depthColors[answer.depth] ?? depthColors[3]}`}>
          [{depthLabels[answer.depth] ?? 'Unknown'}]
        </span>
      </div>
      {answer.user_prompt && (
        <blockquote className="border-l-4 border-gray-300 pl-3 italic text-gray-600 mb-3 text-sm">
          You: {answer.user_prompt}
        </blockquote>
      )}
      <p className="text-gray-800 whitespace-pre-wrap">{answer.ai_response}</p>
      <div className="mt-3">
        <VoteButtons score={score} upvotes={answer.upvotes} downvotes={answer.downvotes} onVote={onVote} onRemoveVote={onRemoveVote} />
      </div>
    </div>
  )
}

function ArgueSection({ questionId, onArgued }: { questionId: string; onArgued: () => void }) {
  const [argument, setArgument] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { user } = useAuth()
  const router = useRouter()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!user) {
      router.push('/login')
      return
    }
    if (!argument.trim()) return
    setSubmitting(true)
    try {
      await argueQuestion(questionId, argument)
      setArgument('')
      onArgued()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to argue with the AI')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="bg-orange-50 border border-orange-200 rounded-lg p-4">
      <h3 className="font-bold text-orange-800 mb-2">Argue with the AI</h3>
      <p className="text-sm text-orange-700 mb-3">
        Think the AI is wrong? Push back and it will escalate its responses!
      </p>
      {error && <p className="text-red-500 text-sm mb-2">{error}</p>}
      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="text"
          value={argument}
          onChange={e => setArgument(e.target.value)}
          placeholder="I disagree because..."
          className="flex-1 border rounded-md px-3 py-2 text-sm"
          disabled={submitting}
        />
        <button
          type="submit"
          disabled={submitting || !argument.trim()}
          className="bg-orange-500 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-orange-600 disabled:opacity-50 transition-colors"
        >
          {submitting ? 'Arguing...' : 'Argue'}
        </button>
      </form>
    </div>
  )
}
