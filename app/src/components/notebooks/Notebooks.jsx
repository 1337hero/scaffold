import { useQuery } from '@tanstack/react-query'
import { NotebookCard } from './NotebookCard.jsx'
import { notebooksQuery } from '@/api/queries.js'

export function Notebooks({ onOpen }) {
  const { data: notebooks = [], isLoading } = useQuery(notebooksQuery)

  if (isLoading) return null

  return (
    <div class="max-w-[760px] mx-auto px-8 py-8 pb-[100px] max-md:px-4 max-md:py-5">
      <div class="flex justify-between items-center mb-6">
        <h2 class="text-[1.4rem] font-bold tracking-tight">Notebooks</h2>
      </div>

      {notebooks.map((nb) => (
        <NotebookCard key={nb.id} notebook={nb} onOpen={() => onOpen(nb.id)} />
      ))}
    </div>
  )
}
