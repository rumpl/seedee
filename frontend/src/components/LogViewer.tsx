import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useVirtualizer } from "@tanstack/react-virtual"
import Ansi from "ansi-to-react"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import { EventType } from "@/gen/seedee/v1/seedee_pb"
import type { LogEntry } from "@/hooks/useLogStream"
import { filterEntries } from "@/hooks/useLogStream"
import type { LogFilterOptions } from "@/hooks/useLogStream"

// ---------------------------------------------------------------------------
// Color palette for job / step prefixes
// ---------------------------------------------------------------------------

const PREFIX_COLORS = [
  "text-cyan-400",
  "text-yellow-400",
  "text-green-400",
  "text-pink-400",
  "text-purple-400",
  "text-orange-400",
  "text-blue-400",
  "text-lime-400",
]

function prefixColor(name: string, palette: Map<string, string>): string {
  if (!palette.has(name)) {
    palette.set(name, PREFIX_COLORS[palette.size % PREFIX_COLORS.length])
  }
  return palette.get(name)!
}

// ---------------------------------------------------------------------------
// LogLine — renders a single row
// ---------------------------------------------------------------------------

interface LogLineProps {
  entry: LogEntry
  colorMap: Map<string, string>
  showTimestamp: boolean
}

function LogLine({ entry, colorMap, showTimestamp }: LogLineProps) {
  const isLifecycle = entry.eventType !== EventType.STEP_LOG

  // Build prefix: [job] [step]
  const prefixParts: { text: string; color: string }[] = []
  if (entry.jobName) {
    prefixParts.push({
      text: entry.jobName,
      color: prefixColor(entry.jobName, colorMap),
    })
  }
  if (entry.stepName) {
    prefixParts.push({
      text: entry.stepName,
      color: prefixColor(
        `${entry.jobName}/${entry.stepName}`,
        colorMap,
      ),
    })
  }

  return (
    <div
      className={`flex items-start gap-2 px-3 py-0.5 leading-5 font-mono text-xs hover:bg-white/5 ${
        isLifecycle
          ? "bg-white/[0.03] text-blue-300 italic"
          : entry.isStderr
            ? "text-red-400"
            : "text-gray-200"
      }`}
    >
      {/* Timestamp */}
      {showTimestamp && (
        <span className="shrink-0 text-gray-600 select-none w-20 text-[10px]">
          {entry.timestamp.slice(11, 23)}
        </span>
      )}

      {/* Prefix badges */}
      {prefixParts.length > 0 && (
        <span className="shrink-0 select-none flex gap-1">
          {prefixParts.map((p, i) => (
            <span
              key={i}
              className={`${p.color} font-bold text-[10px]`}
            >
              [{p.text}]
            </span>
          ))}
        </span>
      )}

      {/* Log text with ANSI color support */}
      <span className="min-w-0 break-all whitespace-pre-wrap">
        <Ansi>{entry.text}</Ansi>
      </span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// LogViewer component
// ---------------------------------------------------------------------------

interface LogViewerProps {
  /** All log entries (unfiltered). */
  entries: LogEntry[]
  /** Available job names for the filter dropdown. */
  jobNames: string[]
  /** Available step names for the filter dropdown. */
  stepNames: string[]
  /** Whether the stream is still active. */
  streaming: boolean
}

export function LogViewer({
  entries,
  jobNames,
  stepNames,
  streaming,
}: LogViewerProps) {
  // ------ Filter state ------
  const [jobFilter, setJobFilter] = useState("")
  const [stepFilter, setStepFilter] = useState("")
  const [showStderr, setShowStderr] = useState(true)
  const [searchQuery, setSearchQuery] = useState("")
  const [showTimestamp, setShowTimestamp] = useState(false)

  // ------ Auto-scroll ------
  const [autoScroll, setAutoScroll] = useState(true)
  const parentRef = useRef<HTMLDivElement>(null)

  const filterOpts: LogFilterOptions = useMemo(
    () => ({ jobFilter, stepFilter, showStderr, searchQuery }),
    [jobFilter, stepFilter, showStderr, searchQuery],
  )

  const filtered = useMemo(
    () => filterEntries(entries, filterOpts),
    [entries, filterOpts],
  )

  // Color map stable across renders.
  const colorMap = useRef(new Map<string, string>()).current

  // ------ Virtualizer ------
  const virtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 20,
    overscan: 30,
  })

  // ------ Auto-scroll logic ------
  useEffect(() => {
    if (autoScroll && filtered.length > 0) {
      virtualizer.scrollToIndex(filtered.length - 1, { align: "end" })
    }
  }, [filtered.length, autoScroll, virtualizer])

  // Detect manual scroll to disable auto-scroll.
  const handleScroll = useCallback(() => {
    const el = parentRef.current
    if (!el) return
    const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    // If user scrolled more than 80px from bottom, disable auto-scroll.
    if (distFromBottom > 80) {
      setAutoScroll(false)
    } else {
      setAutoScroll(true)
    }
  }, [])

  const scrollToBottom = useCallback(() => {
    setAutoScroll(true)
    if (filtered.length > 0) {
      virtualizer.scrollToIndex(filtered.length - 1, { align: "end" })
    }
  }, [filtered.length, virtualizer])

  // Reset step filter when job filter changes.
  useEffect(() => {
    setStepFilter("")
  }, [jobFilter])

  // Compute step names scoped to selected job.
  const scopedStepNames = useMemo(() => {
    if (!jobFilter) return stepNames
    const names = new Set<string>()
    for (const e of entries) {
      if (e.jobName === jobFilter && e.stepName) {
        names.add(e.stepName)
      }
    }
    return Array.from(names)
  }, [jobFilter, stepNames, entries])

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              📟 Logs
              {streaming && (
                <span className="inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse" />
              )}
            </CardTitle>
            <span className="retro text-[10px] text-muted-foreground">
              {filtered.length} / {entries.length} lines
            </span>
          </div>

          {/* Toolbar */}
          <div className="flex flex-wrap items-center gap-2">
            {/* Job filter */}
            <select
              value={jobFilter}
              onChange={(e) => setJobFilter(e.target.value)}
              className="retro text-[10px] h-7 px-2 bg-background border-2 border-foreground dark:border-ring focus:outline-none focus:ring-1 focus:ring-primary"
            >
              <option value="">All Jobs</option>
              {jobNames.map((j) => (
                <option key={j} value={j}>
                  {j}
                </option>
              ))}
            </select>

            {/* Step filter */}
            <select
              value={stepFilter}
              onChange={(e) => setStepFilter(e.target.value)}
              className="retro text-[10px] h-7 px-2 bg-background border-2 border-foreground dark:border-ring focus:outline-none focus:ring-1 focus:ring-primary"
            >
              <option value="">All Steps</option>
              {scopedStepNames.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>

            {/* Stderr toggle */}
            <label className="retro text-[10px] flex items-center gap-1 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={showStderr}
                onChange={(e) => setShowStderr(e.target.checked)}
                className="accent-red-500"
              />
              stderr
            </label>

            {/* Timestamp toggle */}
            <label className="retro text-[10px] flex items-center gap-1 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={showTimestamp}
                onChange={(e) => setShowTimestamp(e.target.checked)}
                className="accent-primary"
              />
              time
            </label>

            {/* Search */}
            <input
              type="text"
              placeholder="🔍 Search logs..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="retro text-[10px] h-7 px-2 flex-1 min-w-[120px] max-w-[240px] bg-background border-2 border-foreground dark:border-ring focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        </div>
      </CardHeader>
      <CardContent className="p-0">
        <div className="relative">
          {/* Terminal viewport */}
          <div
            ref={parentRef}
            onScroll={handleScroll}
            className="h-[420px] overflow-auto bg-[#0d1117] rounded-none"
          >
            <div
              style={{
                height: `${virtualizer.getTotalSize()}px`,
                width: "100%",
                position: "relative",
              }}
            >
              {virtualizer.getVirtualItems().map((virtualRow) => {
                const entry = filtered[virtualRow.index]
                return (
                  <div
                    key={virtualRow.key}
                    data-index={virtualRow.index}
                    ref={virtualizer.measureElement}
                    style={{
                      position: "absolute",
                      top: 0,
                      left: 0,
                      width: "100%",
                      transform: `translateY(${virtualRow.start}px)`,
                    }}
                  >
                    <LogLine
                      entry={entry}
                      colorMap={colorMap}
                      showTimestamp={showTimestamp}
                    />
                  </div>
                )
              })}
            </div>

            {/* Empty state */}
            {filtered.length === 0 && (
              <div className="flex items-center justify-center h-full">
                <span className="retro text-xs text-gray-500">
                  {entries.length === 0
                    ? streaming
                      ? "⏳ Waiting for logs..."
                      : "No log output"
                    : "No logs match current filters"}
                </span>
              </div>
            )}
          </div>

          {/* Scroll-to-bottom button */}
          {!autoScroll && (
            <div className="absolute bottom-3 right-3">
              <Button
                variant="outline"
                size="sm"
                onClick={scrollToBottom}
                className="text-[10px] bg-[#0d1117] border-gray-600 text-gray-300 hover:bg-gray-800"
              >
                ↓ Scroll to bottom
              </Button>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
