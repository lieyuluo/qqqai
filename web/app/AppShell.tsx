'use client'

import Link from 'next/link'
import { BarChart3, FileUp, LogOut, MessageSquare } from 'lucide-react'
import { clearToken } from '@/lib/api'

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <nav className="nav">
        <div className="brand">QQQAI</div>
        <Link href="/chat">
          <MessageSquare size={18} /> Chat
        </Link>
        <Link href="/files">
          <FileUp size={18} /> Files
        </Link>
        <Link href="/admin">
          <BarChart3 size={18} /> Admin
        </Link>
        <button
          onClick={() => {
            clearToken()
            window.location.href = '/login'
          }}
        >
          <LogOut size={18} /> Logout
        </button>
      </nav>
      <main className="main">{children}</main>
    </div>
  )
}
