const EmptyState = ({ message, icon }) => {
  return (
    <div class="flex flex-col items-center justify-center py-14 text-text-muted">
      {icon && <div class="text-3xl mb-3 opacity-40">{icon}</div>}
      <p class="text-[0.98rem]">{message}</p>
    </div>
  )
}

export default EmptyState
