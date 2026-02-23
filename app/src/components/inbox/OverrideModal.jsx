import { useState, useRef, useEffect } from 'preact/hooks'

const typeOptions = ['Todo', 'Note', 'Idea', 'Event', 'Identity', 'Goal', 'Decision', 'Preference', 'Fact', 'Observation']
const actionOptions = ['do', 'explore', 'reference', 'waiting']

const OverrideModal = ({ item, onConfirm, onClose }) => {
  const dialogRef = useRef(null)
  const [type, setType] = useState('Todo')
  const [action, setAction] = useState(item?.triageAction || 'reference')
  const [importance, setImportance] = useState('0.8')
  const [tags, setTags] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!item) return
    setType('Todo')
    setAction(item.triageAction || 'reference')
    setImportance('0.8')
    setTags('')
    setError('')
    const dialog = dialogRef.current
    if (dialog && !dialog.open) dialog.showModal()
    return () => { if (dialog?.open) dialog.close() }
  }, [item])

  function handleCancel() {
    setError('')
    onClose()
  }

  function handleSubmit(e) {
    e.preventDefault()
    const imp = Number(importance)
    if (!Number.isFinite(imp) || imp < 0 || imp > 1) {
      setError('Importance must be a number between 0 and 1')
      return
    }

    const parsedTags = tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)

    setError('')
    onConfirm({
      type: type.trim(),
      action: action.trim().toLowerCase(),
      importance: imp,
      tags: parsedTags,
    })
  }

  if (!item) return null

  return (
    <dialog
      ref={dialogRef}
      onClose={handleCancel}
      class="bg-transparent p-0 m-0 max-w-none max-h-none w-screen h-screen backdrop:bg-black/60 backdrop:backdrop-blur-[4px]"
    >
      <div class="w-full h-full flex items-center justify-center" onClick={(e) => { if (e.target === e.currentTarget) handleCancel() }}>
        <div class="bg-surface border border-border rounded-[16px] p-6 w-[90%] max-w-[480px] shadow-[0_18px_50px_rgba(0,0,0,0.45)]">
          <div class="mb-5">
            <h3 class="text-[1.05rem] font-semibold text-text mb-1">Override Classification</h3>
            <p class="text-[0.85rem] text-text-dim leading-[1.45] line-clamp-2">{item.title || item.text}</p>
          </div>

          <form onSubmit={handleSubmit} class="flex flex-col gap-3.5">
            <label class="flex flex-col gap-1.5">
              <span class="text-[0.78rem] font-medium text-text-dim">Type</span>
              <select
                value={type}
                onChange={(e) => setType(e.currentTarget.value)}
                class="py-2.5 px-3 bg-surface-2 border border-border rounded-[8px] text-text font-sans text-[0.9rem] focus:border-amber appearance-none cursor-pointer"
              >
                {typeOptions.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </label>

            <label class="flex flex-col gap-1.5">
              <span class="text-[0.78rem] font-medium text-text-dim">Action</span>
              <select
                value={action}
                onChange={(e) => setAction(e.currentTarget.value)}
                class="py-2.5 px-3 bg-surface-2 border border-border rounded-[8px] text-text font-sans text-[0.9rem] focus:border-amber appearance-none cursor-pointer"
              >
                {actionOptions.map((a) => <option key={a} value={a}>{a}</option>)}
              </select>
            </label>

            <label class="flex flex-col gap-1.5">
              <span class="text-[0.78rem] font-medium text-text-dim">Importance</span>
              <input
                type="number"
                min="0"
                max="1"
                step="0.1"
                value={importance}
                onInput={(e) => setImportance(e.currentTarget.value)}
                class="py-2.5 px-3 bg-surface-2 border border-border rounded-[8px] text-text font-sans text-[0.9rem] focus:border-amber"
              />
            </label>

            <label class="flex flex-col gap-1.5">
              <span class="text-[0.78rem] font-medium text-text-dim">Tags <span class="text-text-muted font-normal">(comma-separated, optional)</span></span>
              <input
                type="text"
                value={tags}
                onInput={(e) => setTags(e.currentTarget.value)}
                placeholder="work, urgent, project-x"
                class="py-2.5 px-3 bg-surface-2 border border-border rounded-[8px] text-text font-sans text-[0.9rem] focus:border-amber"
              />
            </label>

            {error && (
              <div class="rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-[0.78rem] text-red-300">
                {error}
              </div>
            )}

            <div class="mt-2 flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={handleCancel}
                class="text-[0.78rem] font-medium py-1.5 px-3 rounded-md border border-border text-text-dim hover:bg-surface-3 hover:text-text font-sans cursor-pointer transition-all"
              >
                Cancel
              </button>
              <button
                type="submit"
                class="btn-amber text-[0.78rem] py-1.5 px-3 rounded-md"
              >
                Confirm
              </button>
            </div>
          </form>
        </div>
      </div>
    </dialog>
  )
}

export default OverrideModal
