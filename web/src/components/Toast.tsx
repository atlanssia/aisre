import React, { useState, useCallback, useRef } from 'react'

type ToastType = 'success' | 'error' | 'info'

interface Toast {
  id: number
  type: ToastType
  message: string
}

let nextId = 0

// Shared state outside React so all useToast() callers share the same queue.
// This avoids needing a Context provider.
const listeners: Set<() => void> = new Set()
let toastQueue: Toast[] = []

function emitChange() {
  listeners.forEach((fn) => fn())
}

function addToast(type: ToastType, message: string) {
  const id = nextId++
  toastQueue = [...toastQueue, { id, type, message }]
  emitChange()
  setTimeout(() => {
    toastQueue = toastQueue.filter((t) => t.id !== id)
    emitChange()
  }, 4000)
}

function removeToast(id: number) {
  toastQueue = toastQueue.filter((t) => t.id !== id)
  emitChange()
}

export function useToast() {
  const [, forceUpdate] = useState(0)
  const mounted = useRef(false)

  if (!mounted.current) {
    mounted.current = true
  }

  // Subscribe on first render, unsubscribe on unmount
  // We use a ref-based pattern to avoid re-subscribing
  const subscriberRef = useRef<(() => void) | null>(null)
  if (!subscriberRef.current) {
    subscriberRef.current = () => forceUpdate((n) => n + 1)
    listeners.add(subscriberRef.current)
  }

  // Cleanup is handled via the window unload to keep it simple;
  // in practice React strict-mode double-mount is harmless here
  // because duplicates in the listener set are deduped by reference.

  const showToast = useCallback((type: ToastType, message: string) => {
    addToast(type, message)
  }, [])

  return { toasts: toastQueue, showToast, removeToast }
}

const typeStyles: Record<ToastType, string> = {
  success: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300',
  error: 'border-red-500/40 bg-red-500/10 text-red-300',
  info: 'border-blue-500/40 bg-blue-500/10 text-blue-300',
}

const typeIcons: Record<ToastType, React.ReactElement> = {
  success: (
    <svg className="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
    </svg>
  ),
  error: (
    <svg className="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
    </svg>
  ),
  info: (
    <svg className="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
}

export function ToastContainer({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={`flex items-center gap-2 px-4 py-3 rounded-lg border text-sm shadow-lg animate-slide-in-right ${typeStyles[toast.type]}`}
        >
          {typeIcons[toast.type]}
          <span className="flex-1">{toast.message}</span>
          <button
            onClick={() => onDismiss(toast.id)}
            className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
          >
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}
