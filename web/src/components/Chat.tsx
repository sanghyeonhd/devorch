import { useState, useRef, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Send, Bot, User, Loader2, AlertCircle, Copy, Check, RefreshCw } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  created_at: string
  tool_calls?: ToolCall[]
}

interface ToolCall {
  id: string
  name: string
  arguments: string
  result?: string
}

export default function Chat() {
  const { sessionId } = useParams()
  const [input, setInput] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [streamingContent, setStreamingContent] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const queryClient = useQueryClient()

  const { data: messages = [], isLoading } = useQuery<Message[]>({
    queryKey: ['messages', sessionId],
    queryFn: async () => {
      if (!sessionId) return []
      const res = await fetch(`/api/sessions/${sessionId}/messages`)
      return res.json()
    },
    enabled: !!sessionId,
  })

  const sendMessage = useMutation({
    mutationFn: async (content: string) => {
      setIsStreaming(true)
      setStreamingContent('')

      const res = await fetch(`/api/sessions/${sessionId}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
      })

      if (!res.ok) throw new Error('Failed to send message')

      // Handle streaming response
      const reader = res.body?.getReader()
      const decoder = new TextDecoder()

      if (reader) {
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          const chunk = decoder.decode(value)
          setStreamingContent((prev) => prev + chunk)
        }
      }

      setIsStreaming(false)
      return true
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['messages', sessionId] })
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
    },
    onError: () => {
      setIsStreaming(false)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || isStreaming) return

    const content = input.trim()
    setInput('')
    sendMessage.mutate(content)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit(e)
    }
  }

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  // Auto-resize textarea
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.style.height = 'auto'
      inputRef.current.style.height = `${inputRef.current.scrollHeight}px`
    }
  }, [input])

  if (!sessionId) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="text-center">
          <Bot className="w-16 h-16 mx-auto mb-4 text-devorch-400 opacity-50" />
          <h2 className="text-2xl font-bold text-gradient mb-2">Welcome to DevOrch</h2>
          <p className="text-gray-500 max-w-md">
            Your AI-powered development assistant. Start a new chat or select an existing session.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-8 h-8 animate-spin text-devorch-400" />
          </div>
        ) : messages.length === 0 && !isStreaming ? (
          <div className="flex items-center justify-center py-8">
            <div className="text-center text-gray-500">
              <Bot className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p>Start the conversation by sending a message</p>
            </div>
          </div>
        ) : (
          <>
            {messages.map((message) => (
              <MessageBubble key={message.id} message={message} />
            ))}
            {isStreaming && (
              <MessageBubble
                message={{
                  id: 'streaming',
                  role: 'assistant',
                  content: streamingContent || '...',
                  created_at: new Date().toISOString(),
                }}
                isStreaming
              />
            )}
          </>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t border-gray-800 p-4">
        <form onSubmit={handleSubmit} className="max-w-4xl mx-auto">
          <div className="relative flex items-end bg-gray-800 rounded-xl border border-gray-700 focus-within:border-devorch-500 transition-colors">
            <textarea
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type your message... (Shift+Enter for new line)"
              className="flex-1 bg-transparent px-4 py-3 text-gray-100 placeholder-gray-500 resize-none focus:outline-none max-h-40"
              rows={1}
              disabled={isStreaming}
            />
            <button
              type="submit"
              disabled={!input.trim() || isStreaming}
              className="m-2 p-2 bg-devorch-600 hover:bg-devorch-500 disabled:bg-gray-700 rounded-lg transition-colors"
            >
              {isStreaming ? (
                <Loader2 className="w-5 h-5 animate-spin" />
              ) : (
                <Send className="w-5 h-5" />
              )}
            </button>
          </div>
          <p className="text-xs text-gray-500 mt-2 text-center">
            DevOrch can make mistakes. Please verify important information.
          </p>
        </form>
      </div>
    </div>
  )
}

interface MessageBubbleProps {
  message: Message
  isStreaming?: boolean
}

function MessageBubble({ message, isStreaming }: MessageBubbleProps) {
  const [copied, setCopied] = useState(false)

  const copyContent = async () => {
    await navigator.clipboard.writeText(message.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const isUser = message.role === 'user'
  const isSystem = message.role === 'system'

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} fade-in`}>
      <div
        className={`
          max-w-3xl rounded-xl p-4 border
          ${isUser ? 'message-user' : isSystem ? 'message-system' : 'message-assistant'}
        `}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center space-x-2">
            {isUser ? (
              <User className="w-5 h-5 text-devorch-400" />
            ) : isSystem ? (
              <AlertCircle className="w-5 h-5 text-yellow-400" />
            ) : (
              <Bot className="w-5 h-5 text-devorch-400" />
            )}
            <span className="text-sm font-medium text-gray-400">
              {isUser ? 'You' : isSystem ? 'System' : 'DevOrch'}
            </span>
          </div>
          {!isUser && (
            <button
              onClick={copyContent}
              className="p-1 hover:bg-gray-700 rounded transition-colors"
              title="Copy message"
            >
              {copied ? (
                <Check className="w-4 h-4 text-green-400" />
              ) : (
                <Copy className="w-4 h-4 text-gray-500" />
              )}
            </button>
          )}
        </div>

        {/* Content */}
        <div className="prose prose-invert prose-sm max-w-none">
          <ReactMarkdown
            components={{
              code({ node, inline, className, children, ...props }) {
                const match = /language-(\w+)/.exec(className || '')
                const language = match ? match[1] : ''

                if (!inline && language) {
                  return (
                    <div className="code-block my-2">
                      <div className="code-header">
                        <span className="text-xs text-gray-400">{language}</span>
                        <CodeCopyButton code={String(children)} />
                      </div>
                      <SyntaxHighlighter
                        style={oneDark}
                        language={language}
                        PreTag="div"
                        customStyle={{
                          margin: 0,
                          borderRadius: 0,
                          background: 'transparent',
                        }}
                        {...props}
                      >
                        {String(children).replace(/\n$/, '')}
                      </SyntaxHighlighter>
                    </div>
                  )
                }

                return (
                  <code className="px-1 py-0.5 bg-gray-800 rounded text-devorch-300" {...props}>
                    {children}
                  </code>
                )
              },
            }}
          >
            {message.content}
          </ReactMarkdown>
        </div>

        {/* Tool calls */}
        {message.tool_calls && message.tool_calls.length > 0 && (
          <div className="mt-3 space-y-2">
            {message.tool_calls.map((tool) => (
              <ToolCallDisplay key={tool.id} tool={tool} />
            ))}
          </div>
        )}

        {/* Streaming indicator */}
        {isStreaming && (
          <div className="mt-2 typing-indicator">
            <span></span>
            <span></span>
            <span></span>
          </div>
        )}
      </div>
    </div>
  )
}

function CodeCopyButton({ code }: { code: string }) {
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    await navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button
      onClick={copy}
      className="text-xs text-gray-400 hover:text-gray-200 transition-colors"
    >
      {copied ? 'Copied!' : 'Copy'}
    </button>
  )
}

function ToolCallDisplay({ tool }: { tool: ToolCall }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-3 py-2 bg-gray-800/50 hover:bg-gray-800 transition-colors"
      >
        <div className="flex items-center space-x-2">
          <RefreshCw className="w-4 h-4 text-devorch-400" />
          <span className="text-sm font-medium">{tool.name}</span>
        </div>
        <span className="text-xs text-gray-500">{expanded ? '▲' : '▼'}</span>
      </button>
      {expanded && (
        <div className="px-3 py-2 bg-gray-900/50 text-xs">
          <div className="mb-2">
            <span className="text-gray-500">Arguments:</span>
            <pre className="mt-1 text-gray-300 overflow-x-auto">
              {JSON.stringify(JSON.parse(tool.arguments), null, 2)}
            </pre>
          </div>
          {tool.result && (
            <div>
              <span className="text-gray-500">Result:</span>
              <pre className="mt-1 text-gray-300 overflow-x-auto whitespace-pre-wrap">
                {tool.result}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
