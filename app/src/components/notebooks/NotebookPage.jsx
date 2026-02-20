import { useQuery } from '@tanstack/react-query'
import { typeStyles } from '@/data/mock.js'
import { notebookQuery } from '@/api/queries.js'

export function NotebookPage({ notebookId, onBack }) {
  const { data: thread, isLoading } = useQuery(notebookQuery(notebookId))

  if (isLoading || !thread) return null

  const sanitizeHtml = (html) => html // TODO: replace with DOMPurify before connecting real API

  return (
    <div class="flex flex-col h-screen">
      <div class="py-4 px-6 border-b border-border flex items-center gap-4">
        <button
          type="button"
          onClick={onBack}
          class="text-[0.82rem] text-text-dim cursor-pointer py-1.5 px-3 rounded-md border border-border bg-transparent font-sans transition-all hover:bg-surface hover:text-text"
          aria-label="Back to notebooks"
        >
          {'\u2190 Back'}
        </button>
        <span class="text-[1.1rem] font-semibold">{thread.title}</span>
      </div>

      <div class="flex-1 overflow-y-auto py-6 px-6 max-w-[760px] mx-auto w-full">
        {thread.messages.map((msg) => (
          <div key={msg.id} class="mb-[20px]">
            <div class={`text-[0.68rem] font-semibold uppercase tracking-[0.06em] mb-1 ${msg.author === 'you' ? 'text-amber' : 'text-blue'}`}>
              {msg.author === 'you' ? 'Mike' : 'System'}
            </div>
            {msg.html ? (
              <div
                class="text-[0.88rem] text-text-dim leading-[1.7] [&_ul]:pl-5 [&_ul]:mt-2 [&_li]:mb-1.5 [&_strong]:text-text"
                dangerouslySetInnerHTML={{ __html: sanitizeHtml(msg.html) }}
              />
            ) : (
              <div class="text-[0.88rem] text-text-dim leading-[1.7]">{msg.text}</div>
            )}
          </div>
        ))}

        {thread.relatedNodes?.length > 0 && (
          <div class="mt-[20px] pt-[20px] border-t border-border">
            <div class="text-[0.65rem] uppercase tracking-[0.08em] text-text-muted mb-2.5">
              Related Nodes
            </div>
            {thread.relatedNodes.map((node) => (
              <div
                key={node.id}
                class="bg-surface border border-border flex items-center gap-2.5 py-[10px] px-[14px] rounded-lg mb-1 cursor-pointer transition-all hover:border-border-light"
              >
                <span class={`text-[0.55rem] font-semibold uppercase tracking-[0.06em] py-[2px] px-1.5 rounded-sm shrink-0 ${typeStyles[node.type] || 'bg-surface-2 text-text-dim'}`}>
                  {node.type}
                </span>
                <div>
                  <div class="text-[0.82rem] font-medium leading-tight">{node.title}</div>
                  <div class="text-[0.62rem] text-text-muted mt-0.5">{node.date}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div class="py-4 px-6 border-t border-border">
        <input
          type="text"
          class="w-full max-w-[760px] mx-auto block py-3.5 px-[18px] bg-surface-2 border border-border rounded-[10px] text-text font-sans text-[0.88rem] focus:border-amber"
          placeholder="Ask a question, add context, extract tasks..."
        />
      </div>
    </div>
  )
}
