'use client'

import { FormEvent, useEffect, useState } from 'react'
import { Trash2, Upload } from 'lucide-react'
import { AppShell } from '../AppShell'
import { api, FileRecord } from '@/lib/api'

export default function FilesPage() {
  const [files, setFiles] = useState<FileRecord[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    void loadFiles()
  }, [])

  async function loadFiles() {
    const data = await api<FileRecord[]>('/api/files')
    setFiles(data)
  }

  async function onUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const form = new FormData(event.currentTarget)
    try {
      await api<FileRecord>('/api/files/upload', {
        method: 'POST',
        body: form,
      })
      event.currentTarget.reset()
      await loadFiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    }
  }

  async function remove(id: number) {
    await api(`/api/files/${id}`, { method: 'DELETE' })
    await loadFiles()
  }

  return (
    <AppShell>
      <h1>Files</h1>
      <form onSubmit={onUpload} style={{ display: 'flex', gap: 10, marginBottom: 18 }}>
        <input className="input" name="file" type="file" required />
        <button className="btn" type="submit">
          <Upload size={18} /> Upload
        </button>
      </form>
      <div className="error">{error}</div>
      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Status</th>
            <th>Chunks</th>
            <th>Size</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {files.map((file) => (
            <tr key={file.id}>
              <td>{file.original_name}</td>
              <td>{file.status}</td>
              <td>{file.indexed_count}</td>
              <td>{Math.ceil(file.size / 1024)} KB</td>
              <td>
                <button className="btn danger" onClick={() => void remove(file.id)}>
                  <Trash2 size={18} />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </AppShell>
  )
}
