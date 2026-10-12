import { Loader2 } from 'lucide-react'

export function LoadingState({ message = 'Memuat…' }) {
  return (
    <div className="queue-state">
      <Loader2 size={22} className="spin" />
      <span>{message}</span>
    </div>
  )
}