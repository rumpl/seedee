import { useState, useCallback, useMemo } from "react"
import { useParams, Link } from "react-router"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import { Badge } from "@/components/ui/8bit/badge"
import { ScrollArea } from "@/components/ui/8bit/scroll-area"
import { Separator } from "@/components/ui/8bit/separator"
import { usePipelineStatus, useCancelPipeline } from "@/hooks/usePipelines"
import type { JobStatus, StepStatus } from "@/gen/seedee/v1/seedee_pb"
import { Status } from "@/gen/seedee/v1/seedee_pb"
import type { Duration } from "@bufbuild/protobuf/wkt"
import {
  ChevronDown,
  ChevronRight,
  Clock,
  Circle,
  CheckCircle2,
  XCircle,
  Ban,
  SkipForward,
  Loader2,
  Terminal,
} from "lucide-react"

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function statusLabel(s: Status): string {
  switch (s) {
    case Status.PENDING:
      return "Pending"
    case Status.RUNNING:
      return "Running"
    case Status.SUCCESS:
      return "Success"
    case Status.FAILED:
      return "Failed"
    case Status.SKIPPED:
      return "Skipped"
    case Status.CANCELED:
      return "Canceled"
    default:
      return "Unknown"
  }
}

function statusVariant(
  s: Status,
): "default" | "secondary" | "destructive" | "outline" {
  switch (s) {
    case Status.SUCCESS:
      return "default"
    case Status.RUNNING:
      return "secondary"
    case Status.FAILED:
      return "destructive"
    case Status.CANCELED:
      return "outline"
    default:
      return "secondary"
  }
}

function StatusIcon({ status, className }: { status: Status; className?: string }) {
  const base = className ?? "size-4"
  switch (status) {
    case Status.PENDING:
      return <Circle className={base} />
    case Status.RUNNING:
      return <Loader2 className={`${base} animate-spin`} />
    case Status.SUCCESS:
      return <CheckCircle2 className={`${base} text-green-600`} />
    case Status.FAILED:
      return <XCircle className={`${base} text-red-600`} />
    case Status.CANCELED:
      return <Ban className={`${base} text-orange-500`} />
    case Status.SKIPPED:
      return <SkipForward className={`${base} text-muted-foreground`} />
    default:
      return <Circle className={`${base} text-muted-foreground`} />
  }
}

function formatDuration(d: Duration | undefined): string {
  if (!d) return "—"
  const totalSeconds = Number(d.seconds) + d.nanos / 1_000_000_000
  if (totalSeconds < 1) {
    const ms = Math.round(totalSeconds * 1000)
    return `${ms}ms`
  }
  if (totalSeconds < 60) {
    return `${totalSeconds.toFixed(1)}s`
  }
  const mins = Math.floor(totalSeconds / 60)
  const secs = Math.round(totalSeconds % 60)
  return `${mins}m ${secs}s`
}

function isTerminalStatus(s: Status): boolean {
  return (
    s === Status.SUCCESS ||
    s === Status.FAILED ||
    s === Status.CANCELED ||
    s === Status.SKIPPED
  )
}

// ---------------------------------------------------------------------------
// GanttBar — horizontal bar for timeline view
// ---------------------------------------------------------------------------

function GanttBar({
  job,
  maxDurationSecs,
}: {
  job: JobStatus
  maxDurationSecs: number
}) {
  const jobSecs = job.duration
    ? Number(job.duration.seconds) + job.duration.nanos / 1_000_000_000
    : 0
  const widthPct = maxDurationSecs > 0 ? (jobSecs / maxDurationSecs) * 100 : 0

  const barColor = (() => {
    switch (job.status) {
      case Status.SUCCESS:
        return "bg-green-600"
      case Status.FAILED:
        return "bg-red-600"
      case Status.RUNNING:
        return "bg-blue-500 animate-pulse"
      case Status.CANCELED:
        return "bg-orange-500"
      case Status.SKIPPED:
        return "bg-muted-foreground/40"
      default:
        return "bg-muted-foreground/20"
    }
  })()

  return (
    <div className="flex items-center gap-3 py-1">
      <span className="retro text-xs w-32 truncate shrink-0">{job.name}</span>
      <div className="flex-1 h-5 bg-muted relative">
        <div
          className={`h-full ${barColor} transition-all duration-300`}
          style={{ width: `${Math.max(widthPct, 2)}%` }}
        />
      </div>
      <span className="retro text-xs text-muted-foreground w-16 text-right shrink-0">
        {formatDuration(job.duration)}
      </span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// StepRow — a single step inside a job accordion
// ---------------------------------------------------------------------------

function StepRow({ step }: { step: StepStatus }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border-l-2 border-dashed border-muted-foreground/30 ml-3 pl-3">
      <button
        type="button"
        className="flex items-center gap-2 w-full text-left py-1.5 hover:bg-muted/50 transition-colors px-1"
        onClick={() => setExpanded((v) => !v)}
      >
        {expanded ? (
          <ChevronDown className="size-3 shrink-0" />
        ) : (
          <ChevronRight className="size-3 shrink-0" />
        )}
        <StatusIcon status={step.status} className="size-3.5" />
        <span className="retro text-xs flex-1 truncate">{step.name}</span>
        {step.exitCode !== 0 && isTerminalStatus(step.status) && (
          <Badge variant="destructive" className="text-[8px] px-1.5">
            exit {step.exitCode}
          </Badge>
        )}
        <span className="retro text-[10px] text-muted-foreground shrink-0">
          {formatDuration(step.duration)}
        </span>
      </button>

      {expanded && (
        <div className="ml-5 mb-2 mt-1">
          <div className="bg-foreground text-primary-foreground p-3 font-mono text-xs">
            <div className="flex items-center gap-2 mb-2 text-muted-foreground">
              <Terminal className="size-3" />
              <span className="retro text-[10px]">Log output</span>
            </div>
            <ScrollArea className="max-h-48">
              <pre className="text-xs whitespace-pre-wrap break-all text-primary-foreground/80">
                {`$ ${step.name}\n`}
                {step.status === Status.RUNNING
                  ? "⏳ Running..."
                  : step.status === Status.PENDING
                    ? "⏸ Waiting to start..."
                    : step.exitCode === 0
                      ? "✓ Completed successfully"
                      : `✗ Exited with code ${step.exitCode}`}
              </pre>
            </ScrollArea>
          </div>
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// JobCard — expandable accordion for a single job
// ---------------------------------------------------------------------------

function JobCard({ job }: { job: JobStatus }) {
  const [expanded, setExpanded] = useState(
    job.status === Status.RUNNING || job.status === Status.FAILED,
  )

  return (
    <Card className="mb-3">
      <button
        type="button"
        className="w-full text-left"
        onClick={() => setExpanded((v) => !v)}
      >
        <CardHeader>
          <div className="flex items-center gap-3">
            {expanded ? (
              <ChevronDown className="size-4 shrink-0" />
            ) : (
              <ChevronRight className="size-4 shrink-0" />
            )}
            <StatusIcon status={job.status} />
            <span className="retro text-sm flex-1 truncate">{job.name}</span>
            <Badge variant={statusVariant(job.status)} className="text-[10px]">
              {statusLabel(job.status)}
            </Badge>
            <div className="flex items-center gap-1 text-muted-foreground shrink-0">
              <Clock className="size-3" />
              <span className="retro text-[10px]">
                {formatDuration(job.duration)}
              </span>
            </div>
          </div>
        </CardHeader>
      </button>

      {expanded && (
        <CardContent>
          {job.steps.length === 0 ? (
            <p className="retro text-xs text-muted-foreground">
              No steps recorded
            </p>
          ) : (
            <div className="space-y-1">
              {job.steps.map((step) => (
                <StepRow key={step.name} step={step} />
              ))}
            </div>
          )}
        </CardContent>
      )}
    </Card>
  )
}

// ---------------------------------------------------------------------------
// PipelineDetail — main page component
// ---------------------------------------------------------------------------

export default function PipelineDetail() {
  const { id } = useParams<{ id: string }>()
  const pipelineId = id ?? ""

  // Track whether we've reached a terminal state to stop polling
  const [stopped, setStopped] = useState(false)

  // Auto-refresh: poll every 2s, stop on terminal state
  const { data: pipelineData, error, loading } = usePipelineStatus({
    pipelineId,
    intervalMs: 2000,
    enabled: !stopped,
  })

  // Stop polling once we reach a terminal state
  if (pipelineData && isTerminalStatus(pipelineData.status) && !stopped) {
    setStopped(true)
  }

  const { cancel, loading: cancelling } = useCancelPipeline()

  const handleCancel = useCallback(async () => {
    if (!pipelineId) return
    try {
      await cancel(pipelineId)
    } catch {
      // Error is surfaced via useCancelPipeline().error
    }
  }, [cancel, pipelineId])

  // Compute max duration for Gantt chart
  const maxDurationSecs = useMemo(() => {
    if (!pipelineData?.jobs) return 0
    return pipelineData.jobs.reduce((max, job) => {
      const secs = job.duration
        ? Number(job.duration.seconds) + job.duration.nanos / 1_000_000_000
        : 0
      return Math.max(max, secs)
    }, 0)
  }, [pipelineData?.jobs])

  const showTimeline =
    pipelineData?.jobs && pipelineData.jobs.length > 0

  const [view, setView] = useState<"jobs" | "timeline">("jobs")

  return (
    <div className="container mx-auto py-8 px-4">
      {/* Navigation */}
      <div className="mb-6">
        <Link to="/">
          <Button variant="ghost" size="sm" className="mb-4">
            ← Back to Dashboard
          </Button>
        </Link>

        {/* Pipeline Header */}
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div className="flex items-center gap-3">
            <h1 className="retro text-xl font-bold tracking-tight">
              {pipelineData?.pipelineName || pipelineId}
            </h1>
            {pipelineData && (
              <Badge variant={statusVariant(pipelineData.status)}>
                {statusLabel(pipelineData.status)}
              </Badge>
            )}
          </div>

          <div className="flex items-center gap-4">
            {pipelineData?.duration && (
              <div className="flex items-center gap-1.5 text-muted-foreground">
                <Clock className="size-4" />
                <span className="retro text-xs">
                  {formatDuration(pipelineData.duration)}
                </span>
              </div>
            )}

            {/* Cancel button — only when running */}
            {pipelineData &&
              !isTerminalStatus(pipelineData.status) &&
              pipelineData.status !== Status.UNSPECIFIED && (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleCancel}
                  disabled={cancelling}
                >
                  {cancelling ? (
                    <>
                      <Loader2 className="size-3 animate-spin" />
                      Cancelling…
                    </>
                  ) : (
                    <>
                      <Ban className="size-3" />
                      Cancel
                    </>
                  )}
                </Button>
              )}
          </div>
        </div>

        <p className="retro text-[10px] text-muted-foreground mt-2">
          ID: {pipelineId}
        </p>
      </div>

      <Separator className="mb-6" />

      {/* Loading / Error states */}
      {loading && !pipelineData && (
        <Card>
          <CardContent className="py-12">
            <div className="flex flex-col items-center gap-3">
              <Loader2 className="size-8 animate-spin text-muted-foreground" />
              <p className="retro text-xs text-muted-foreground">
                Loading pipeline…
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {error && !pipelineData && (
        <Card>
          <CardContent className="py-12">
            <div className="flex flex-col items-center gap-3">
              <XCircle className="size-8 text-red-600" />
              <p className="retro text-xs text-red-600">{error.message}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Main content */}
      {pipelineData && (
        <>
          {/* View toggle */}
          <div className="flex gap-2 mb-4">
            <Button
              variant={view === "jobs" ? "default" : "outline"}
              size="sm"
              onClick={() => setView("jobs")}
            >
              Jobs
            </Button>
            {showTimeline && (
              <Button
                variant={view === "timeline" ? "default" : "outline"}
                size="sm"
                onClick={() => setView("timeline")}
              >
                Timeline
              </Button>
            )}
          </div>

          {/* Jobs view */}
          {view === "jobs" && (
            <div>
              {pipelineData.jobs.length === 0 ? (
                <Card>
                  <CardContent className="py-8">
                    <p className="retro text-xs text-muted-foreground text-center">
                      No jobs yet
                    </p>
                  </CardContent>
                </Card>
              ) : (
                pipelineData.jobs.map((job) => (
                  <JobCard key={job.name} job={job} />
                ))
              )}
            </div>
          )}

          {/* Timeline / Gantt view */}
          {view === "timeline" && showTimeline && (
            <Card>
              <CardHeader>
                <CardTitle className="retro text-sm">
                  Job Timeline
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-1">
                  {pipelineData.jobs.map((job) => (
                    <GanttBar
                      key={job.name}
                      job={job}
                      maxDurationSecs={maxDurationSecs}
                    />
                  ))}
                </div>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  )
}
