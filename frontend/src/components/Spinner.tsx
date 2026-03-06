import { cn } from "@/lib/utils"

interface SpinnerProps {
  className?: string
  /** Accessible label for screen readers */
  label?: string
}

/**
 * Retro pixel-style loading spinner.
 */
export function Spinner({ className, label = "Loading…" }: SpinnerProps) {
  return (
    <span
      role="status"
      aria-label={label}
      className={cn("inline-flex items-center gap-1.5", className)}
    >
      <span className="inline-block w-3 h-3 border-2 border-foreground border-t-transparent dark:border-ring dark:border-t-transparent animate-spin" />
      <span className="sr-only">{label}</span>
    </span>
  )
}
