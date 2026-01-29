import { MessageSquare, Clock, Trash2, X } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'

interface Session {
  id: string
  title: string
  created_at: string
  updated_at: string
  message_count: number
}

interface SidebarProps {
  open: boolean
  onClose: () => void
}

export default function Sidebar({ open, onClose }: SidebarProps) {
  const location = useLocation()
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
    const date = new Date(dateStr)
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const days = Math.floor(diff / (1000 * 60 * 60 * 24))

    if (days === 0) return 'Today'
    if (days === 1) return 'Yesterday'
    if (days < 7) return `${days} days ago`
    return date.toLocaleDateString()
  }

  return (
    <>
      {/* Overlay for mobile */}
      {open && (
        <div
          className="fixed inset-0 bg-black/50 z-20 lg:hidden"
          onClick={onClose}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed lg:static inset-y-0 left-0 z-30
          w-72 bg-gray-900 border-r border-gray-800
          transform transition-transform duration-200 ease-in-out
          ${open ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
          flex flex-col
        `}
      >
        {/* Header */}
        <div className="h-14 border-b border-gray-800 flex items-center justify-between px-4">
          <h2 className="text-lg font-semibold text-gray-200">Chat History</h2>
          <button
            onClick={onClose}
            className="btn-ghost p-1 rounded lg:hidden"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Session List */}
        <nav className="flex-1 overflow-y-auto p-2 space-y-1">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <div className="typing-indicator">
                <span></span>
                <span></span>
                <span></span>
              </div>
            </div>
          ) : sessions.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              <MessageSquare className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p>No chat sessions yet</p>
              <p className="text-sm">Start a new chat to begin</p>
            </div>
          ) : (
            sessions.map((session) => {
              const isActive = location.pathname === `/chat/${session.id}`
              return (
                <div
                  key={session.id}
                  className={`
                    group flex items-center rounded-lg p-3
                    transition-colors duration-150
                    ${isActive 
                      ? 'bg-devorch-900/50 border border-devorch-700' 
                      : 'hover:bg-gray-800 border border-transparent'
                    }
                  `}
                >
                  <Link
                    to={`/chat/${session.id}`}
                    className="flex-1 min-w-0"
                    onClick={onClose}
                  >
                    <p className="text-sm font-medium text-gray-200 truncate">
                      {session.title}
                    </p>
                    <div className="flex items-center text-xs text-gray-500 mt-1">
                      <Clock className="w-3 h-3 mr-1" />
                      <span>{formatDate(session.updated_at)}</span>
                      <span className="mx-2">•</span>
                      <span>{session.message_count} messages</span>
                    </div>
                  </Link>
                  <button
                    onClick={() => deleteSession.mutate(session.id)}
                    className="p-1 opacity-0 group-hover:opacity-100 hover:text-red-400 transition-all"
                    aria-label="Delete session"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              )
            })
          )}
        </nav>

        {/* Footer */}
        <div className="p-4 border-t border-gray-800">
          <p className="text-xs text-gray-500 text-center">
            DevOrch v2.0 • Powered by AI
          </p>
        </div>
      </aside>
    </>
  )
}
