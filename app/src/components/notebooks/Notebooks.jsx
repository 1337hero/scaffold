import { useQuery } from '@tanstack/react-query'
import { NotebookCard } from './NotebookCard.jsx'
import { notebooksQuery } from '@/api/queries.js'

export function Notebooks({ onOpen }) {
  const { data: notebooks = [], isLoading } = useQuery(notebooksQuery)

  if (isLoading) return null

  return (
    <div class="panel-shell">
      <div class="flex justify-between items-center mb-8">
        <h2 class="panel-title">Notebooks</h2>
      </div>

      {notebooks.map((nb) => (
        <NotebookCard key={nb.id} notebook={nb} onOpen={() => onOpen(nb.id)} />
      ))}
    </div>
  )
}
