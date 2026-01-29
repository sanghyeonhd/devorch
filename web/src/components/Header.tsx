import { Menu, Plus, Settings, Terminal, Sun, Moon } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { useState } from 'react'

interface HeaderProps {
  onMenuClick: () => void
}

export default function Header({ onMenuClick }: HeaderProps) {
  const navigate = useNavigate()
  const [darkMode, setDarkMode] = useState(true)

  const toggleDarkMode = () => {
    setDarkMode(!darkMode)
    document.documentElement.classList.toggle('dark')
  }

  const handleNewChat = async () => {
    try {
      const res = await fetch('/api/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: 'New Chat' }),
      })
      const session = await res.json()
      navigate(`/chat/${session.id}`)
    } catch (error) {
      console.error('Failed to create session:', error)
    }
  }

  return (
    <header className="h-14 border-b border-gray-800 bg-gray-900/50 backdrop-blur-sm flex items-center justify-between px-4">
      <div className="flex items-center space-x-4">
        <button
          onClick={onMenuClick}
          className="btn-ghost p-2 rounded-lg lg:hidden"
          aria-label="Toggle menu"
        >
          <Menu className="w-5 h-5" />
        </button>

        <Link to="/" className="flex items-center space-x-2">
          <Terminal className="w-6 h-6 text-devorch-400" />
          <span className="text-xl font-bold text-gradient">DevOrch</span>
        </Link>
      </div>

      <div className="flex items-center space-x-2">
        <button
          onClick={handleNewChat}
          className="btn-primary flex items-center space-x-2"
        >
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">New Chat</span>
        </button>

        <button
          onClick={toggleDarkMode}
          className="btn-ghost p-2 rounded-lg"
          aria-label="Toggle dark mode"
        >
          {darkMode ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
        </button>

        <Link to="/settings" className="btn-ghost p-2 rounded-lg">
          <Settings className="w-5 h-5" />
        </Link>
      </div>
    </header>
  )
}
