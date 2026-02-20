import { colorClass } from '../../data/mock.js'

export function NotebookCard({ notebook, onOpen }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      class="w-full text-left surface-card rounded-xl p-6 mb-2.5 cursor-pointer flex gap-4.5 transition-all"
      aria-label={`Open ${notebook.title} notebook`}
    >
      <div class={`w-11 h-11 rounded-[11px] flex items-center justify-center text-[1.15rem] shrink-0 ${colorClass(notebook.iconBg, 'bg')}`}>
        {notebook.icon}
      </div>

      <div class="flex-1">
        <div class="text-[1.1rem] leading-tight font-semibold mb-1.5">{notebook.title}</div>
        <div class="text-[0.92rem] text-text-dim mb-2.5">{notebook.desc}</div>
        <div class="flex gap-4 text-[0.78rem] text-text-muted">
          <span>{notebook.nodes} nodes</span>
          <span>{notebook.updated}</span>
        </div>
      </div>
    </button>
  )
}
