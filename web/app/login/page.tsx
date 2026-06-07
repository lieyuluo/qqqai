'use client'

import Link from 'next/link'
import { LogIn } from 'lucide-react'
import { FormEvent, useState } from 'react'
import { api, setToken, User } from '@/lib/api'

export default function LoginPage() {
  const [error, setError] = useState('')

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const form = new FormData(event.currentTarget)
    try {
      const data = await api<{ token: string; user: User }>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          email: form.get('email'),
          password: form.get('password'),
        }),
      })
      setToken(data.token)
      window.location.href = '/chat'
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    }
  }

  return (
    <main className="auth-page">
      <form className="panel" onSubmit={onSubmit}>
        <h1>Login</h1>
        <div className="field">
          <label>Email</label>
          <input className="input" name="email" type="email" required />
        </div>
        <div className="field">
          <label>Password</label>
          <input className="input" name="password" type="password" required />
        </div>
        <div className="error">{error}</div>
        <button className="btn" type="submit">
          <LogIn size={18} /> Login
        </button>
        <p>
          <Link href="/register">Create account</Link>
        </p>
      </form>
    </main>
  )
}
