import { useState, useEffect } from 'preact/hooks'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Sidebar } from './components/Sidebar.jsx'
import { MobileBar } from './components/MobileBar.jsx'
import { CaptureModal } from './components/CaptureModal.jsx'
import { Desk } from './components/desk/Desk.jsx'
import { Inbox } from './components/inbox/Inbox.jsx'
import { Map } from './components/map/Map.jsx'
import { DomainDetail } from './components/map/DomainDetail.jsx'
import { Login } from './components/Login.jsx'
import { inboxQuery } from '@/api/queries.js'

function AppShell() {
  const [activePanel, setActivePanel] = useState('desk')
  const [activeDomain, setActiveDomain] = useState(null)
  const [captureOpen, setCaptureOpen] = useState(false)

  const { data: inboxGroups = [] } = useQuery(inboxQuery)
  const inboxCount = inboxGroups.reduce((sum, g) => sum + g.items.length, 0)

  const openCapture = () => setCaptureOpen(true)
  const closeCapture = () => setCaptureOpen(false)

  const navigateTo = (panel) => {
    setActivePanel(panel)
    setActiveDomain(null)
  }

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
        {activeDomain !== null ? (
          <DomainDetail domainId={activeDomain} onBack={() => setActiveDomain(null)} />
        ) : (
          <>
            {activePanel === 'desk' && <Desk />}
            {activePanel === 'inbox' && <Inbox />}
            {activePanel === 'map' && <Map onOpenDomain={(id) => setActiveDomain(id)} />}
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
  const queryClient = useQueryClient()

  const { data: authed, isLoading } = useQuery({
    queryKey: ['auth'],
    queryFn: () =>
      fetch('/api/auth/check', { credentials: 'include' })
        .then((res) => res.ok)
        .catch(() => false),
    retry: false,
    staleTime: Infinity,
  })

  useEffect(() => {
    const onExpired = () => queryClient.setQueryData(['auth'], false)
    window.addEventListener('auth:expired', onExpired)
    return () => window.removeEventListener('auth:expired', onExpired)
  }, [queryClient])

  if (isLoading) {
    return (
      <div class="flex items-center justify-center min-h-screen bg-bg">
        <div class="w-6 h-6 border-2 border-text/20 border-t-text/60 rounded-full animate-spin" />
      </div>
    )
  }

  if (!authed) {
    return <Login onSuccess={() => queryClient.setQueryData(['auth'], true)} />
  }

  return <AppShell />
}
