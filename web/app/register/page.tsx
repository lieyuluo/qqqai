'use client'

import Link from 'next/link'
import { UserPlus } from 'lucide-react'
import { FormEvent, useState } from 'react'
import { api } from '@/lib/api'

export default function RegisterPage() {
  const [error, setError] = useState('')

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const form = new FormData(event.currentTarget)
    try {
      await api('/api/auth/register', {
        method: 'POST',
        body: JSON.stringify({
          email: form.get('email'),
          username: form.get('username'),
          password: form.get('password'),
        }),
      })
      window.location.href = '/login'
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Register failed')
    }
  }

  return (
    <main className="auth-page">
      <form className="panel" onSubmit={onSubmit}>
        <h1>Register</h1>
        <div className="field">
          <label>Email</label>
          <input className="input" name="email" type="email" required />
        </div>
        <div className="field">
          <label>Username</label>
          <input className="input" name="username" required />
        </div>
        <div className="field">
          <label>Password</label>
          <input className="input" name="password" type="password" minLength={6} required />
        </div>
        <div className="error">{error}</div>
        <button className="btn" type="submit">
          <UserPlus size={18} /> Register
        </button>
        <p>
          <Link href="/login">Back to login</Link>
        </p>
      </form>
    </main>
  )
}
