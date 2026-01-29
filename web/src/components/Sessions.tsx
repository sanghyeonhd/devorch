import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { MessageSquare, Clock, Trash2, Search, Filter } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useState } from 'react'

interface Session {
  id: string
  title: string
  created_at: string
  updated_at: string
  message_count: number
}

export default function Sessions() {
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'updated' | 'created'>('updated')
  const queryClient = useQueryClient()

  const { data: sessions = [], isLoading } = useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: async () => {
      const res = await fetch('/api/sessions')
      return res.json()
    },
  })

  const deleteSession = useMutation({
    mutationFn: async (sessionId: string) => {
      await fetch(`/api/sessions/${sessionId}`, { method: 'DELETE' })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
    },
  })

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString('ko-KR', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const filteredSessions = sessions
    .filter((session) =>
      session.title.toLowerCase().includes(searchQuery.toLowerCase())
    )
    .sort((a, b) => {
      const dateA = sortBy === 'updated' ? a.updated_at : a.created_at
      const dateB = sortBy === 'updated' ? b.updated_at : b.created_at
      return new Date(dateB).getTime() - new Date(dateA).getTime()
    })

  return (
    <div className="h-full overflow-y-auto p-6">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gradient mb-2">Chat Sessions</h1>
          <p className="text-gray-500">
            Browse and manage your conversation history
          </p>
        </div>

        {/* Search and Filter */}
        <div className="flex flex-col sm:flex-row gap-4 mb-6">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search sessions..."
              className="input pl-10"
            />
          </div>
          <div className="flex items-center space-x-2">
            <Filter className="w-5 h-5 text-gray-500" />
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as 'updated' | 'created')}
              className="input w-auto"
            >
              <option value="updated">Last Updated</option>
              <option value="created">Created Date</option>
            </select>
          </div>
        </div>

        {/* Sessions Grid */}
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="typing-indicator">
              <span></span>
              <span></span>
              <span></span>
            </div>
          </div>
        ) : filteredSessions.length === 0 ? (
          <div className="text-center py-12">
            <MessageSquare className="w-16 h-16 mx-auto mb-4 text-gray-600" />
            {searchQuery ? (
              <>
                <h3 className="text-xl font-medium text-gray-300 mb-2">
                  No matching sessions
                </h3>
                <p className="text-gray-500">
                  Try adjusting your search query
                </p>
              </>
            ) : (
              <>
                <h3 className="text-xl font-medium text-gray-300 mb-2">
                  No sessions yet
                </h3>
                <p className="text-gray-500 mb-4">
                  Start a new chat to create your first session
                </p>
                <Link to="/" className="btn-primary">
                  Start New Chat
                </Link>
              </>
            )}
          </div>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {filteredSessions.map((session) => (
              <div
                key={session.id}
                className="card hover:border-devorch-600 transition-colors group"
              >
                <div className="flex items-start justify-between">
                  <Link
                    to={`/chat/${session.id}`}
                    className="flex-1 min-w-0"
                  >
                    <h3 className="text-lg font-semibold text-gray-200 truncate group-hover:text-devorch-400 transition-colors">
                      {session.title}
                    </h3>
                    <div className="flex items-center text-sm text-gray-500 mt-2">
                      <MessageSquare className="w-4 h-4 mr-1" />
                      <span>{session.message_count} messages</span>
                    </div>
                    <div className="flex items-center text-xs text-gray-600 mt-1">
                      <Clock className="w-3 h-3 mr-1" />
                      <span>Updated {formatDate(session.updated_at)}</span>
                    </div>
                  </Link>
                  <button
                    onClick={() => {
                      if (confirm('Are you sure you want to delete this session?')) {
                        deleteSession.mutate(session.id)
                      }
                    }}
                    className="p-2 opacity-0 group-hover:opacity-100 hover:bg-red-900/50 hover:text-red-400 rounded-lg transition-all"
                    title="Delete session"
                  >
                    <Trash2 className="w-5 h-5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Stats */}
        {sessions.length > 0 && (
          <div className="mt-8 p-4 bg-gray-900/50 rounded-xl border border-gray-800">
            <h4 className="text-sm font-medium text-gray-400 mb-3">Statistics</h4>
            <div className="grid grid-cols-3 gap-4 text-center">
              <div>
                <div className="text-2xl font-bold text-devorch-400">
                  {sessions.length}
                </div>
                <div className="text-xs text-gray-500">Total Sessions</div>
              </div>
              <div>
                <div className="text-2xl font-bold text-devorch-400">
                  {sessions.reduce((sum, s) => sum + s.message_count, 0)}
                </div>
                <div className="text-xs text-gray-500">Total Messages</div>
              </div>
              <div>
                <div className="text-2xl font-bold text-devorch-400">
                  {Math.round(
                    sessions.reduce((sum, s) => sum + s.message_count, 0) /
                      sessions.length
                  ) || 0}
                </div>
                <div className="text-xs text-gray-500">Avg per Session</div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
