import { useState } from 'preact/hooks'

export function Login({ onSuccess }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(false)

  async function onSubmit(e) {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })

      if (res.ok) {
        onSuccess()
        return
      }
      if (res.status === 401) {
        setError('Invalid credentials')
        return
      }
      if (res.status === 429) {
        setError('Too many attempts, try again later')
        return
      }
      setError(`Error ${res.status}`)
    } catch {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div class="min-h-screen bg-bg flex items-center justify-center px-4">
      <div class="bg-surface border border-border rounded-xl p-8 w-full max-w-[400px]">
        <div class="flex items-center gap-2.5 mb-8">
          <div class="w-8 h-8 rounded-md bg-amber-dim border border-amber-border flex items-center justify-center text-[0.9rem] text-amber">
            {'\u26a1'}
          </div>
          <span class="text-[1.1rem] font-bold tracking-tight">Scaffold</span>
        </div>

        <form onSubmit={onSubmit} class="flex flex-col gap-4">
          <input
            type="text"
            value={username}
            onInput={(e) => setUsername(e.currentTarget.value)}
            placeholder="Username"
            autocomplete="username"
            required
            class="w-full py-3 px-[18px] bg-surface-2 border border-border rounded-[10px] text-text font-sans text-base focus:border-amber focus:outline-none"
          />
          <input
            type="password"
            value={password}
            onInput={(e) => setPassword(e.currentTarget.value)}
            placeholder="Password"
            autocomplete="current-password"
            required
            class="w-full py-3 px-[18px] bg-surface-2 border border-border rounded-[10px] text-text font-sans text-base focus:border-amber focus:outline-none"
          />

          {error && (
            <p class="text-[0.82rem] text-red-400">{error}</p>
          )}

          <button
            type="submit"
            disabled={loading}
            class="btn-amber w-full py-2.5 px-4 rounded-md text-[0.82rem]"
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  )
}
