import { Routes, Route } from 'react-router-dom'
import { useState } from 'react'
import Header from './components/Header'
import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import Sessions from './components/Sessions'
import Settings from './components/Settings'

function App() {
  const [sidebarOpen, setSidebarOpen] = useState(true)

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />

      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0">
        <Header onMenuClick={() => setSidebarOpen(!sidebarOpen)} />
        
        <main className="flex-1 overflow-hidden">
          <Routes>
            <Route path="/" element={<Chat />} />
            <Route path="/chat/:sessionId" element={<Chat />} />
            <Route path="/sessions" element={<Sessions />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

export default App
