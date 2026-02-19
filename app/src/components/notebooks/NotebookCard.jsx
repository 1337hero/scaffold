import { colorClass } from '../../data/mock.js'

export function NotebookCard({ notebook, onOpen }) {
  return (
    <button
      onClick={onOpen}
      class="w-full text-left bg-surface border border-border rounded-lg p-5 mb-2 cursor-pointer transition-all hover:border-border-light flex gap-4"
      aria-label={`Open ${notebook.title} notebook`}
    >
      <div class={`w-10 h-10 rounded-[10px] flex items-center justify-center text-[1.1rem] shrink-0 ${colorClass(notebook.iconBg, 'bg')}`}>
        {notebook.icon}
      </div>

      <div class="flex-1">
        <div class="text-base font-semibold mb-[3px]">{notebook.title}</div>
        <div class="text-[0.8rem] text-text-dim mb-2">{notebook.desc}</div>
        <div class="flex gap-3.5 text-[0.7rem] text-text-muted">
          <span>{notebook.nodes} nodes</span>
          <span>{notebook.updated}</span>
        </div>
      </div>
    </button>
  )
}
