import { Toaster as SonnerToaster } from "sonner"

/**
 * Retro-styled toast container. Drop this once in the app root.
 */
export function Toaster() {
  return (
    <SonnerToaster
      position="bottom-right"
      toastOptions={{
        className: "retro !text-xs !rounded-none !border-2 !border-foreground dark:!border-ring !shadow-none",
        style: {
          fontFamily: "'Press Start 2P', system-ui, sans-serif",
          fontSize: "10px",
          lineHeight: "1.5",
        },
      }}
    />
  )
}
