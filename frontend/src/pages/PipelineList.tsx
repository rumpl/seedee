import { useNavigate } from "react-router"
import { useState, useEffect } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/8bit/table"
import { useListPipelines } from "@/hooks/usePipelines"
import { StatusBadge } from "@/components/StatusBadge"
import { statusIcon } from "@/lib/status"
import { RunPipelineDialog } from "@/components/RunPipelineDialog"
import { Spinner } from "@/components/Spinner"
import type { PipelineSummary } from "@/gen/seedee/v1/seedee_pb"

function formatDuration(duration?: {
  seconds?: bigint
  nanos?: number
}): string {
  if (!duration?.seconds && !duration?.nanos) return "—"
  const totalSeconds = Number(duration.seconds ?? 0n)
  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}m ${seconds}s`
}

function formatTimestamp(timestamp?: {
  seconds?: bigint
  nanos?: number
}): string {
  if (!timestamp?.seconds) return "—"
  const date = new Date(Number(timestamp.seconds) * 1000)
  return date.toLocaleString()
}

function shortId(id: string): string {
  if (id.length <= 12) return id
  return id.slice(0, 8) + "…"
}

// ---------- Loading Skeleton ----------
function LoadingSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label="Loading pipelines">
      {/* Desktop skeleton */}
      <div className="hidden md:block">
        <div className="w-full">
          {[...Array(5)].map((_, i) => (
            <div
              key={i}
              className="flex gap-4 py-3 px-2 border-b border-dashed border-muted animate-pulse"
              style={{ animationDelay: `${i * 100}ms` }}
            >
              <div className="h-4 w-16 bg-muted rounded" />
              <div className="h-4 w-32 bg-muted rounded" />
              <div className="h-4 w-20 bg-muted rounded" />
              <div className="h-4 w-16 bg-muted rounded" />
              <div className="h-4 w-24 bg-muted rounded" />
            </div>
          ))}
        </div>
      </div>
      {/* Mobile skeleton */}
      <div className="md:hidden grid gap-4">
        {[...Array(3)].map((_, i) => (
          <Card key={i}>
            <CardContent className="pt-4">
              <div
                className="space-y-3 animate-pulse"
                style={{ animationDelay: `${i * 120}ms` }}
              >
                <div className="h-4 w-3/4 bg-muted rounded" />
                <div className="h-3 w-1/2 bg-muted rounded" />
                <div className="h-3 w-2/3 bg-muted rounded" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
      <span className="sr-only">Loading pipeline data…</span>
    </div>
  )
}

// ---------- Empty State ----------
function EmptyState({ onRunPipeline }: { onRunPipeline: () => void }) {
  return (
    <Card>
      <CardContent className="py-12">
        <div className="text-center space-y-4">
          <div className="text-4xl" aria-hidden="true">
            🏗️
          </div>
          <p className="retro text-sm text-muted-foreground">
            No pipeline runs yet
          </p>
          <p className="retro text-xs text-muted-foreground">
            Run your first pipeline to see it here!
          </p>
          <Button size="sm" onClick={onRunPipeline}>
            🚀 Run Pipeline
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

// ---------- Mobile Card ----------
function PipelineCard({
  pipeline,
  onClick,
}: {
  pipeline: PipelineSummary
  onClick: () => void
}) {
  return (
    <Card>
      <CardHeader
        className="cursor-pointer"
        onClick={onClick}
        tabIndex={0}
        role="link"
        aria-label={`Pipeline ${pipeline.name || "Unnamed"}`}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault()
            onClick()
          }
        }}
      >
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm">
            <span aria-hidden="true">{statusIcon(pipeline.status)}</span>{" "}
            {pipeline.name || "Unnamed Pipeline"}
          </CardTitle>
          <StatusBadge status={pipeline.status} />
        </div>
      </CardHeader>
      <CardContent className="cursor-pointer" onClick={onClick}>
        <div className="grid grid-cols-2 gap-2 text-xs">
          <div>
            <span className="retro text-muted-foreground">ID: </span>
            <span className="retro font-mono">
              {shortId(pipeline.pipelineId)}
            </span>
          </div>
          <div>
            <span className="retro text-muted-foreground">Duration: </span>
            <span className="retro">{formatDuration(pipeline.duration)}</span>
          </div>
          <div className="col-span-2">
            <span className="retro text-muted-foreground">Started: </span>
            <span className="retro">
              {formatTimestamp(pipeline.startedAt)}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ---------- Desktop Table ----------
function PipelineTable({
  pipelines,
  onRowClick,
}: {
  pipelines: PipelineSummary[]
  onRowClick: (id: string) => void
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="retro text-xs">Status</TableHead>
          <TableHead className="retro text-xs">Pipeline Name</TableHead>
          <TableHead className="retro text-xs">ID</TableHead>
          <TableHead className="retro text-xs">Duration</TableHead>
          <TableHead className="retro text-xs">Started At</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {pipelines.map((p) => (
          <TableRow
            key={p.pipelineId}
            className="cursor-pointer hover:bg-accent/50 transition-colors duration-150 focus-within:bg-accent/50"
            onClick={() => onRowClick(p.pipelineId)}
            tabIndex={0}
            role="link"
            aria-label={`Pipeline ${p.name || "Unnamed"} — ${p.pipelineId}`}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault()
                onRowClick(p.pipelineId)
              }
            }}
          >
            <TableCell>
              <StatusBadge status={p.status} />
            </TableCell>
            <TableCell className="retro text-xs font-medium">
              {p.name || "Unnamed Pipeline"}
            </TableCell>
            <TableCell className="retro text-xs font-mono text-muted-foreground">
              {shortId(p.pipelineId)}
            </TableCell>
            <TableCell className="retro text-xs">
              {formatDuration(p.duration)}
            </TableCell>
            <TableCell className="retro text-xs text-muted-foreground">
              {formatTimestamp(p.startedAt)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// ---------- Main Page ----------
export default function PipelineList() {
  const { data, error, loading } = useListPipelines({ intervalMs: 5000 })
  const [dialogOpen, setDialogOpen] = useState(false)
  const navigate = useNavigate()

  const pipelines = data?.pipelines ?? []

  // Toast on error
  useEffect(() => {
    if (error) {
      toast.error("Failed to load pipelines", {
        description: error.message,
      })
    }
  }, [error])

  const handleRowClick = (id: string) => {
    navigate(`/pipeline/${id}`)
  }

  return (
    <div className="container mx-auto py-8 px-4 animate-in fade-in duration-300">
      {/* Page header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-8 gap-4">
        <div>
          <h1 className="retro text-2xl font-bold tracking-tight">
            Dashboard
          </h1>
          <p className="retro text-xs text-muted-foreground mt-2">
            Monitor your CI/CD pipelines
          </p>
        </div>
        <Button size="sm" onClick={() => setDialogOpen(true)}>
          🚀 Run Pipeline
        </Button>
      </div>

      {/* Error state */}
      {error && (
        <Card className="mb-4" role="alert">
          <CardContent className="py-4">
            <div className="flex items-center justify-between">
              <p className="retro text-xs text-red-500">
                ⚠️ Failed to load pipelines: {error.message}
              </p>
              <Spinner
                className="text-red-500"
                label="Retrying connection…"
              />
            </div>
          </CardContent>
        </Card>
      )}

      {/* Loading state */}
      {loading && <LoadingSkeleton />}

      {/* Empty state */}
      {!loading && !error && pipelines.length === 0 && (
        <EmptyState onRunPipeline={() => setDialogOpen(true)} />
      )}

      {/* Pipeline list */}
      {!loading && pipelines.length > 0 && (
        <>
          {/* Desktop table */}
          <div className="hidden md:block">
            <PipelineTable
              pipelines={pipelines}
              onRowClick={handleRowClick}
            />
          </div>

          {/* Mobile cards */}
          <div className="md:hidden grid gap-4">
            {pipelines.map((p, i) => (
              <div
                key={p.pipelineId}
                className="animate-in fade-in slide-in-from-bottom-2 duration-300"
                style={{ animationDelay: `${i * 60}ms`, animationFillMode: "both" }}
              >
                <PipelineCard
                  pipeline={p}
                  onClick={() => handleRowClick(p.pipelineId)}
                />
              </div>
            ))}
          </div>
        </>
      )}

      {/* Run Pipeline Dialog */}
      <RunPipelineDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
      />
    </div>
  )
}
