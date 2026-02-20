import { colorClass } from '../../data/mock.js'

export function NotebookCard({ notebook, onOpen }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      class="w-full text-left surface-card rounded-xl p-5 mb-2 cursor-pointer flex gap-4 transition-all"
      aria-label={`Open ${notebook.title} notebook`}
    >
      <div class={`w-10 h-10 rounded-[10px] flex items-center justify-center text-[1.1rem] shrink-0 ${colorClass(notebook.iconBg, 'bg')}`}>
        {notebook.icon}
      </div>

      <div class="flex-1">
        <div class="text-[1rem] leading-tight font-semibold mb-1">{notebook.title}</div>
        <div class="text-[0.8rem] text-text-dim mb-2">{notebook.desc}</div>
        <div class="flex gap-3.5 text-[0.7rem] text-text-muted">
          <span>{notebook.nodes} nodes</span>
          <span>{notebook.updated}</span>
        </div>
      </div>
    </button>
  )
}
