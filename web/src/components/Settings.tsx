import { useState, useEffect } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { 
  Save, 
  Key, 
  Palette, 
  Server, 
  Globe, 
  Check, 
  AlertCircle,
  Eye,
  EyeOff,
  RefreshCw
} from 'lucide-react'

interface Settings {
  provider: string
  model: string
  theme: string
  api_keys: {
    openai?: string
    anthropic?: string
    google?: string
    groq?: string
    together?: string
    mistral?: string
    azure_endpoint?: string
    azure_key?: string
    azure_deployment?: string
  }
  preferences: {
    stream_responses: boolean
    show_tool_calls: boolean
    auto_approve_tools: boolean
    max_context_length: number
  }
}

const PROVIDERS = [
  { id: 'openai', name: 'OpenAI', models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-3.5-turbo'] },
  { id: 'anthropic', name: 'Anthropic', models: ['claude-3-5-sonnet-20241022', 'claude-3-opus-20240229', 'claude-3-haiku-20240307'] },
  { id: 'google', name: 'Google AI', models: ['gemini-2.0-flash-exp', 'gemini-1.5-pro', 'gemini-1.5-flash'] },
  { id: 'groq', name: 'Groq', models: ['llama-3.3-70b-versatile', 'mixtral-8x7b-32768', 'gemma2-9b-it'] },
  { id: 'together', name: 'Together AI', models: ['meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo', 'meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo'] },
  { id: 'mistral', name: 'Mistral AI', models: ['mistral-large-latest', 'codestral-latest', 'mistral-small-latest'] },
  { id: 'azure', name: 'Azure OpenAI', models: ['gpt-4o', 'gpt-4'] },
  { id: 'ollama', name: 'Ollama (Local)', models: ['llama3.2', 'codellama', 'mistral'] },
]

const THEMES = [
  { id: 'dark', name: 'Dark' },
  { id: 'light', name: 'Light' },
  { id: 'dracula', name: 'Dracula' },
  { id: 'nord', name: 'Nord' },
  { id: 'gruvbox-dark', name: 'Gruvbox Dark' },
  { id: 'gruvbox-light', name: 'Gruvbox Light' },
  { id: 'tokyo-night', name: 'Tokyo Night' },
  { id: 'catppuccin-mocha', name: 'Catppuccin Mocha' },
  { id: 'monokai', name: 'Monokai' },
  { id: 'solarized-dark', name: 'Solarized Dark' },
  { id: 'rose-pine', name: 'Rosé Pine' },
  { id: 'one-dark', name: 'One Dark' },
]

export default function Settings() {
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({})
  const [saved, setSaved] = useState(false)
  const [localSettings, setLocalSettings] = useState<Settings | null>(null)

  const { data: settings, isLoading, refetch } = useQuery<Settings>({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await fetch('/api/settings')
      return res.json()
    },
  })

  useEffect(() => {
    if (settings && !localSettings) {
      setLocalSettings(settings)
    }
  }, [settings])

  const saveSettings = useMutation({
    mutationFn: async (newSettings: Settings) => {
      const res = await fetch('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newSettings),
      })
      if (!res.ok) throw new Error('Failed to save settings')
      return res.json()
    },
    onSuccess: () => {
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
      refetch()
    },
  })

  const handleSave = () => {
    if (localSettings) {
      saveSettings.mutate(localSettings)
    }
  }

  const updateSetting = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    if (localSettings) {
      setLocalSettings({ ...localSettings, [key]: value })
    }
  }

  const updateApiKey = (provider: string, value: string) => {
    if (localSettings) {
      setLocalSettings({
        ...localSettings,
        api_keys: { ...localSettings.api_keys, [provider]: value },
      })
    }
  }

  const updatePreference = (key: keyof Settings['preferences'], value: any) => {
    if (localSettings) {
      setLocalSettings({
        ...localSettings,
        preferences: { ...localSettings.preferences, [key]: value },
      })
    }
  }

  const toggleShowKey = (key: string) => {
    setShowKeys((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const selectedProvider = PROVIDERS.find((p) => p.id === localSettings?.provider)

  if (isLoading || !localSettings) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="typing-indicator">
          <span></span>
          <span></span>
          <span></span>
        </div>
      </div>
    )
  }

  return (
    <div className="h-full overflow-y-auto p-6">
      <div className="max-w-3xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold text-gradient mb-2">Settings</h1>
            <p className="text-gray-500">
              Configure DevOrch to your preferences
            </p>
          </div>
          <button
            onClick={handleSave}
            disabled={saveSettings.isPending}
            className="btn-primary flex items-center space-x-2"
          >
            {saveSettings.isPending ? (
              <RefreshCw className="w-4 h-4 animate-spin" />
            ) : saved ? (
              <Check className="w-4 h-4" />
            ) : (
              <Save className="w-4 h-4" />
            )}
            <span>{saved ? 'Saved!' : 'Save Settings'}</span>
          </button>
        </div>

        {/* Provider Settings */}
        <section className="card mb-6">
          <h2 className="flex items-center text-lg font-semibold mb-4">
            <Server className="w-5 h-5 mr-2 text-devorch-400" />
            AI Provider
          </h2>
          
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Provider
              </label>
              <select
                value={localSettings.provider}
                onChange={(e) => updateSetting('provider', e.target.value)}
                className="input"
              >
                {PROVIDERS.map((provider) => (
                  <option key={provider.id} value={provider.id}>
                    {provider.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Model
              </label>
              <select
                value={localSettings.model}
                onChange={(e) => updateSetting('model', e.target.value)}
                className="input"
              >
                {selectedProvider?.models.map((model) => (
                  <option key={model} value={model}>
                    {model}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </section>

        {/* API Keys */}
        <section className="card mb-6">
          <h2 className="flex items-center text-lg font-semibold mb-4">
            <Key className="w-5 h-5 mr-2 text-devorch-400" />
            API Keys
          </h2>
          
          <div className="space-y-4">
            {[
              { id: 'openai', label: 'OpenAI API Key' },
              { id: 'anthropic', label: 'Anthropic API Key' },
              { id: 'google', label: 'Google AI API Key' },
              { id: 'groq', label: 'Groq API Key' },
              { id: 'together', label: 'Together AI API Key' },
              { id: 'mistral', label: 'Mistral API Key' },
            ].map(({ id, label }) => (
              <div key={id}>
                <label className="block text-sm font-medium text-gray-400 mb-2">
                  {label}
                </label>
                <div className="relative">
                  <input
                    type={showKeys[id] ? 'text' : 'password'}
                    value={(localSettings.api_keys as any)[id] || ''}
                    onChange={(e) => updateApiKey(id, e.target.value)}
                    placeholder={`Enter your ${label}`}
                    className="input pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => toggleShowKey(id)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
                  >
                    {showKeys[id] ? (
                      <EyeOff className="w-4 h-4" />
                    ) : (
                      <Eye className="w-4 h-4" />
                    )}
                  </button>
                </div>
              </div>
            ))}

            {/* Azure OpenAI specific fields */}
            {localSettings.provider === 'azure' && (
              <>
                <div className="pt-4 border-t border-gray-700">
                  <h3 className="text-sm font-medium text-gray-300 mb-3">
                    Azure OpenAI Configuration
                  </h3>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-2">
                    Azure Endpoint
                  </label>
                  <input
                    type="text"
                    value={localSettings.api_keys.azure_endpoint || ''}
                    onChange={(e) => updateApiKey('azure_endpoint', e.target.value)}
                    placeholder="https://your-resource.openai.azure.com"
                    className="input"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-2">
                    Azure API Key
                  </label>
                  <div className="relative">
                    <input
                      type={showKeys.azure_key ? 'text' : 'password'}
                      value={localSettings.api_keys.azure_key || ''}
                      onChange={(e) => updateApiKey('azure_key', e.target.value)}
                      placeholder="Enter your Azure API key"
                      className="input pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => toggleShowKey('azure_key')}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
                    >
                      {showKeys.azure_key ? (
                        <EyeOff className="w-4 h-4" />
                      ) : (
                        <Eye className="w-4 h-4" />
                      )}
                    </button>
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-2">
                    Deployment Name
                  </label>
                  <input
                    type="text"
                    value={localSettings.api_keys.azure_deployment || ''}
                    onChange={(e) => updateApiKey('azure_deployment', e.target.value)}
                    placeholder="gpt-4o"
                    className="input"
                  />
                </div>
              </>
            )}
          </div>

          <div className="mt-4 p-3 bg-yellow-900/20 border border-yellow-700/30 rounded-lg flex items-start space-x-2">
            <AlertCircle className="w-5 h-5 text-yellow-500 shrink-0 mt-0.5" />
            <p className="text-sm text-yellow-200/80">
              API keys are stored locally and never sent to our servers. You can also set them as environment variables.
            </p>
          </div>
        </section>

        {/* Theme */}
        <section className="card mb-6">
          <h2 className="flex items-center text-lg font-semibold mb-4">
            <Palette className="w-5 h-5 mr-2 text-devorch-400" />
            Appearance
          </h2>
          
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-2">
              Theme
            </label>
            <div className="grid grid-cols-3 sm:grid-cols-4 gap-2">
              {THEMES.map((theme) => (
                <button
                  key={theme.id}
                  onClick={() => updateSetting('theme', theme.id)}
                  className={`
                    p-3 rounded-lg border text-sm font-medium transition-all
                    ${localSettings.theme === theme.id
                      ? 'border-devorch-500 bg-devorch-900/30 text-devorch-400'
                      : 'border-gray-700 hover:border-gray-600 text-gray-400'
                    }
                  `}
                >
                  {theme.name}
                </button>
              ))}
            </div>
          </div>
        </section>

        {/* Preferences */}
        <section className="card mb-6">
          <h2 className="flex items-center text-lg font-semibold mb-4">
            <Globe className="w-5 h-5 mr-2 text-devorch-400" />
            Preferences
          </h2>
          
          <div className="space-y-4">
            <label className="flex items-center justify-between cursor-pointer">
              <div>
                <div className="text-sm font-medium text-gray-300">
                  Stream Responses
                </div>
                <div className="text-xs text-gray-500">
                  Show AI responses as they are generated
                </div>
              </div>
              <input
                type="checkbox"
                checked={localSettings.preferences.stream_responses}
                onChange={(e) => updatePreference('stream_responses', e.target.checked)}
                className="w-5 h-5 rounded bg-gray-700 border-gray-600 text-devorch-500 focus:ring-devorch-500"
              />
            </label>

            <label className="flex items-center justify-between cursor-pointer">
              <div>
                <div className="text-sm font-medium text-gray-300">
                  Show Tool Calls
                </div>
                <div className="text-xs text-gray-500">
                  Display tool invocations in chat
                </div>
              </div>
              <input
                type="checkbox"
                checked={localSettings.preferences.show_tool_calls}
                onChange={(e) => updatePreference('show_tool_calls', e.target.checked)}
                className="w-5 h-5 rounded bg-gray-700 border-gray-600 text-devorch-500 focus:ring-devorch-500"
              />
            </label>

            <label className="flex items-center justify-between cursor-pointer">
              <div>
                <div className="text-sm font-medium text-gray-300">
                  Auto-approve Tool Calls
                </div>
                <div className="text-xs text-gray-500">
                  Automatically execute safe tool calls
                </div>
              </div>
              <input
                type="checkbox"
                checked={localSettings.preferences.auto_approve_tools}
                onChange={(e) => updatePreference('auto_approve_tools', e.target.checked)}
                className="w-5 h-5 rounded bg-gray-700 border-gray-600 text-devorch-500 focus:ring-devorch-500"
              />
            </label>

            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">
                Max Context Length
              </label>
              <input
                type="number"
                value={localSettings.preferences.max_context_length}
                onChange={(e) => updatePreference('max_context_length', parseInt(e.target.value))}
                min={1000}
                max={200000}
                step={1000}
                className="input"
              />
              <p className="text-xs text-gray-500 mt-1">
                Maximum tokens to include in context (1,000 - 200,000)
              </p>
            </div>
          </div>
        </section>

        {/* Version Info */}
        <div className="text-center text-sm text-gray-600">
          <p>DevOrch v2.0.0 • Web UI</p>
          <p className="mt-1">
            <a href="https://github.com/devorch/devorch" className="hover:text-devorch-400">
              GitHub
            </a>
            {' • '}
            <a href="https://devorch.dev/docs" className="hover:text-devorch-400">
              Documentation
            </a>
          </p>
        </div>
      </div>
    </div>
  )
}
