'use client'

import { useEffect, useState } from 'react'
import { AppShell } from '../AppShell'
import { AdminStats, api } from '@/lib/api'

const labels: Record<keyof AdminStats, string> = {
  users: 'Users',
  conversations: 'Conversations',
  messages: 'Messages',
  files: 'Files',
  model_calls: 'Model calls',
  indexed_files: 'Indexed files',
  failed_files: 'Failed files',
}

export default function AdminPage() {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api<AdminStats>('/api/admin/stats')
      .then(setStats)
      .catch((err) => setError(err instanceof Error ? err.message : 'Load failed'))
  }, [])

  return (
    <AppShell>
      <h1>Admin</h1>
      <div className="error">{error}</div>
      {stats && (
        <div className="stat-grid">
          {(Object.keys(labels) as Array<keyof AdminStats>).map((key) => (
            <div className="stat" key={key}>
              <span>{labels[key]}</span>
              <strong>{stats[key]}</strong>
            </div>
          ))}
        </div>
      )}
    </AppShell>
  )
}
