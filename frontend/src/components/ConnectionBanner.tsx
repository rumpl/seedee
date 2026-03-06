import { useCallback, useEffect, useState } from "react"

/**
 * Shows a retro banner when the browser goes offline, with a retry action.
 */
export function ConnectionBanner() {
  const [offline, setOffline] = useState(!navigator.onLine)

  useEffect(() => {
    const goOffline = () => setOffline(true)
    const goOnline = () => setOffline(false)
    window.addEventListener("offline", goOffline)
    window.addEventListener("online", goOnline)
    return () => {
      window.removeEventListener("offline", goOffline)
      window.removeEventListener("online", goOnline)
    }
  }, [])

  const handleRetry = useCallback(() => {
    window.location.reload()
  }, [])

  if (!offline) return null

  return (
    <div
      role="alert"
      aria-live="assertive"
      className="fixed top-0 inset-x-0 z-50 bg-destructive text-white py-2 px-4 flex items-center justify-center gap-3 animate-in fade-in slide-in-from-top duration-300"
    >
      <span className="retro text-[10px]">
        ⚠️ Connection lost — check your network
      </span>
      <button
        onClick={handleRetry}
        className="retro text-[10px] underline hover:no-underline focus-visible:outline-2 focus-visible:outline-white"
      >
        Retry
      </button>
    </div>
  )
}
