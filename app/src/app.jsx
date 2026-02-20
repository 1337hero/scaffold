import { useState, useEffect } from 'preact/hooks'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { Sidebar } from './components/Sidebar.jsx'
import { MobileBar } from './components/MobileBar.jsx'
import { CaptureModal } from './components/CaptureModal.jsx'
import { Desk } from './components/desk/Desk.jsx'
import { Inbox } from './components/inbox/Inbox.jsx'
import { Notebooks } from './components/notebooks/Notebooks.jsx'
import { NotebookPage } from './components/notebooks/NotebookPage.jsx'
import { Login } from './components/Login.jsx'
import { inboxQuery } from '@/api/queries.js'

const queryClient = new QueryClient()

function AppShell() {
  const [activePanel, setActivePanel] = useState('desk')
  const [activeNotebook, setActiveNotebook] = useState(null)
  const [captureOpen, setCaptureOpen] = useState(false)

  const { data: inboxGroups = [] } = useQuery(inboxQuery)
  const inboxCount = inboxGroups.reduce((sum, g) => sum + g.items.length, 0)

  const openCapture = () => setCaptureOpen(true)
  const closeCapture = () => setCaptureOpen(false)

  const navigateTo = (panel) => {
    setActivePanel(panel)
    setActiveNotebook(null)
  }

  const openNotebook = (id) => setActiveNotebook(id)
  const closeNotebook = () => setActiveNotebook(null)

  useEffect(() => {
    function onKeyDown(e) {
      if (e.key === 'Escape' && captureOpen) {
        closeCapture()
        return
      }
      if (e.key === 'c' && !captureOpen && e.target.tagName !== 'INPUT' && e.target.tagName !== 'TEXTAREA') {
        openCapture()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [captureOpen, openCapture, closeCapture])

  return (
    <div class="flex min-h-screen bg-bg text-text">
      <Sidebar
        activePanel={activePanel}
        onNavigate={navigateTo}
        onCapture={openCapture}
        inboxCount={inboxCount}
      />

      <main class="flex-1 overflow-y-auto h-screen">
        {activeNotebook ? (
          <NotebookPage notebookId={activeNotebook} onBack={closeNotebook} />
        ) : (
          <>
            {activePanel === 'desk' && <Desk />}
            {activePanel === 'inbox' && <Inbox />}
            {activePanel === 'notebooks' && <Notebooks onOpen={openNotebook} />}
          </>
        )}
      </main>

      <MobileBar
        activePanel={activePanel}
        onNavigate={navigateTo}
        onCapture={openCapture}
      />

      <CaptureModal open={captureOpen} onClose={closeCapture} />
    </div>
  )
}

export function App() {
  const [authed, setAuthed] = useState(null)

  useEffect(() => {
    fetch('/api/auth/check', { credentials: 'include' })
      .then((res) => setAuthed(res.ok))
      .catch(() => setAuthed(false))
  }, [])

  useEffect(() => {
    const onExpired = () => setAuthed(false)
    window.addEventListener('auth:expired', onExpired)
    return () => window.removeEventListener('auth:expired', onExpired)
  }, [])

  if (authed === null) return null

  if (!authed) {
    return <Login onSuccess={() => setAuthed(true)} />
  }

  return (
    <QueryClientProvider client={queryClient}>
      <AppShell />
    </QueryClientProvider>
  )
}
