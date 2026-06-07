'use client'

import { FormEvent, useEffect, useState } from 'react'
import { Plus, Send } from 'lucide-react'
import { AppShell } from '../AppShell'
import { api, Conversation, Message, streamChat } from '@/lib/api'

export default function ChatPage() {
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [active, setActive] = useState<Conversation | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [draft, setDraft] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    void loadConversations()
  }, [])

  useEffect(() => {
    if (active) void loadMessages(active.id)
  }, [active])

  async function loadConversations() {
    const data = await api<Conversation[]>('/api/conversations')
    setConversations(data)
    if (!active && data.length > 0) setActive(data[0])
  }

  async function loadMessages(id: number) {
    const data = await api<Message[]>(`/api/conversations/${id}/messages?limit=100`)
    setMessages(data)
  }

  async function newConversation(title = 'New conversation') {
    const conversation = await api<Conversation>('/api/conversations', {
      method: 'POST',
      body: JSON.stringify({ title }),
    })
    setConversations((items) => [conversation, ...items])
    setActive(conversation)
    setMessages([])
    return conversation
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    const text = draft.trim()
    if (!text || streaming) return
    setDraft('')
    setError('')
    setStreaming(true)
    try {
      const conversation = active || (await newConversation(text.slice(0, 24)))
      setMessages((items) => [
        ...items,
        {
          id: Date.now(),
          conversation_id: conversation.id,
          user_id: 0,
          role: 'user',
          content: text,
          created_at: new Date().toISOString(),
        },
        {
          id: Date.now() + 1,
          conversation_id: conversation.id,
          user_id: 0,
          role: 'assistant',
          content: '',
          created_at: new Date().toISOString(),
        },
      ])
      await streamChat(conversation.id, text, (chunk) => {
        setMessages((items) => {
          const next = [...items]
          const last = next[next.length - 1]
          if (last?.role === 'assistant') {
            next[next.length - 1] = { ...last, content: last.content + chunk }
          }
          return next
        })
      })
      await loadMessages(conversation.id)
      await loadConversations()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Chat failed')
    } finally {
      setStreaming(false)
    }
  }

  return (
    <AppShell>
      <div className="chat-layout">
        <aside className="sidebar">
          <button className="btn secondary" onClick={() => void newConversation()}>
            <Plus size={18} /> New
          </button>
          <div style={{ height: 12 }} />
          {conversations.map((item) => (
            <button
              key={item.id}
              className={`conversation ${active?.id === item.id ? 'active' : ''}`}
              onClick={() => setActive(item)}
            >
              {item.title}
            </button>
          ))}
        </aside>
        <section className="workspace">
          <div className="messages">
            {messages.map((item) => (
              <div key={item.id} className={`bubble ${item.role}`}>
                {item.content}
              </div>
            ))}
            {error && <div className="error">{error}</div>}
          </div>
          <form className="composer" onSubmit={onSubmit}>
            <textarea
              className="textarea"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              rows={2}
            />
            <button className="btn" disabled={streaming} type="submit">
              <Send size={18} /> Send
            </button>
          </form>
        </section>
      </div>
    </AppShell>
  )
}
