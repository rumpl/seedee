import { useParams, Link } from "react-router"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"
import Layout from "@/components/Layout"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/8bit/tabs"
import { ScrollArea } from "@/components/ui/8bit/scroll-area"
import { Separator } from "@/components/ui/8bit/separator"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/8bit/table"
import { StatusBadge } from "@/components/StatusBadge"
import { LogViewer } from "@/components/LogViewer"
import { Spinner } from "@/components/Spinner"
import { usePipelineStatus, useRunPipeline } from "@/hooks/usePipelines"
import { useLogStream } from "@/hooks/useLogStream"
import { create } from "@bufbuild/protobuf"
import {
  Status,
  PipelineDefinitionSchema,
} from "@/gen/seedee/v1/seedee_pb"
import type { RunPipelineEvent } from "@/gen/seedee/v1/seedee_pb"
import NotFound from "@/pages/NotFound"

// ---------- Loading Skeleton ----------
function DetailSkeleton() {
  return (
    <div
      className="space-y-6 animate-pulse"
      role="status"
      aria-label="Loading pipeline details"
    >
      <div className="flex items-center gap-3">
        <div className="h-7 w-48 bg-muted rounded" />
        <div className="h-5 w-20 bg-muted rounded" />
      </div>
      <div className="h-px bg-muted" />
      <div className="space-y-3">
        {[...Array(3)].map((_, i) => (
          <div
            key={i}
            className="flex gap-4 py-3"
            style={{ animationDelay: `${i * 80}ms` }}
          >
            <div className="h-4 w-24 bg-muted rounded" />
            <div className="h-4 w-20 bg-muted rounded" />
            <div className="h-4 w-16 bg-muted rounded" />
          </div>
        ))}
      </div>
      <span className="sr-only">Loading pipeline details…</span>
    </div>
  )
}

// ---------- Step sub-table ----------
function StepRows({
  steps,
}: {
  steps: { name: string; status: Status; exitCode: number; duration?: { seconds?: bigint } }[]
}) {
  return (
    <>
      {steps.map((step) => (
        <TableRow key={step.name} className="bg-muted/30">
          <TableCell className="retro text-[10px] pl-8 text-muted-foreground">
            ↳ {step.name}
          </TableCell>
          <TableCell>
            <StatusBadge status={step.status} />
          </TableCell>
          <TableCell className="retro text-xs">
            {step.duration
              ? `${Number(step.duration.seconds ?? 0n)}s`
              : "—"}
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}

export default function PipelineDetail() {
  const { id } = useParams<{ id: string }>()
  const { data, loading, error } = usePipelineStatus({
    pipelineId: id ?? "",
    enabled: !!id,
  })

  // --- Log stream state ---
  const logStream = useLogStream()
  const { run, running } = useRunPipeline()
  const [hasLogs, setHasLogs] = useState(false)

  // --- Collapsible job rows ---
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const toggleJob = useCallback((name: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }, [])

  // Toast on error
  useEffect(() => {
    if (error) {
      toast.error("Failed to load pipeline status", {
        description: error.message,
      })
    }
  }, [error])

  // Callback for streaming events into the log viewer.
  const handleEvent = useCallback(
    (event: RunPipelineEvent) => {
      logStream.push(event)
      setHasLogs(true)
    },
    [logStream],
  )

  const handleStartStream = useCallback(async () => {
    if (!data) return
    logStream.reset()
    setHasLogs(true)

    const pipeline = create(PipelineDefinitionSchema, {
      name: data.pipelineName || "pipeline",
      jobs: Object.fromEntries(
        data.jobs.map((j) => [
          j.name,
          {
            $typeName: "seedee.v1.JobDefinition" as const,
            image: "alpine:latest",
            dependsOn: [] as string[],
            env: {} as Record<string, string>,
            steps: j.steps.map((s) => ({
              $typeName: "seedee.v1.StepDefinition" as const,
              name: s.name,
              run: "echo re-run",
              env: {} as Record<string, string>,
            })),
          },
        ]),
      ),
    })

    await run(pipeline, {
      onEvent: handleEvent,
      onComplete: () => {
        logStream.complete()
        toast.success("Pipeline run completed")
      },
      onError: (err) => {
        logStream.complete()
        toast.error("Pipeline run failed", { description: err.message })
      },
    })
  }, [data, run, logStream, handleEvent])

  // Determine if pipeline is/was running (show log viewer tab).
  const isRunning = data?.status === Status.RUNNING
  const showLogs = hasLogs || isRunning

  const defaultTab = useMemo(
    () => (showLogs ? "logs" : "runs"),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  )

  // 404 for unknown pipeline — show after loading completes with no data
  const is404 = !loading && !data && error

  if (is404) {
    return <NotFound />
  }

  return (
    <Layout>
      <div className="container mx-auto py-8 px-4 animate-in fade-in duration-300">
        <div className="mb-6">
          <Link to="/">
            <Button
              variant="ghost"
              size="sm"
              className="mb-4"
              aria-label="Back to dashboard"
            >
              ← Back to Dashboard
            </Button>
          </Link>

          {loading && !data ? (
            <DetailSkeleton />
          ) : (
            <div className="flex items-center gap-3">
              <h1 className="retro text-2xl font-bold tracking-tight">
                Pipeline: {data?.pipelineName || id}
              </h1>
              {data && <StatusBadge status={data.status} />}
            </div>
          )}
        </div>

        <Separator className="mb-6" />

        <Tabs defaultValue={defaultTab}>
          <TabsList aria-label="Pipeline tabs">
            <TabsTrigger value="runs">Jobs</TabsTrigger>
            <TabsTrigger value="logs">
              📟 Logs
              {logStream.streaming && hasLogs && (
                <span
                  className="ml-1 inline-block w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse"
                  aria-label="Live streaming"
                />
              )}
            </TabsTrigger>
            <TabsTrigger value="config">Configuration</TabsTrigger>
          </TabsList>

          {/* Jobs tab */}
          <TabsContent value="runs">
            <Card>
              <CardHeader>
                <CardTitle>Jobs</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="retro text-xs">Job</TableHead>
                      <TableHead className="retro text-xs">Status</TableHead>
                      <TableHead className="retro text-xs">Duration</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data && data.jobs.length > 0 ? (
                      data.jobs.map((job) => (
                        <>
                          <TableRow
                            key={job.name}
                            className="cursor-pointer hover:bg-accent/50 transition-colors duration-150"
                            onClick={() => toggleJob(job.name)}
                            tabIndex={0}
                            role="button"
                            aria-expanded={expanded.has(job.name)}
                            aria-label={`Job ${job.name}`}
                            onKeyDown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault()
                                toggleJob(job.name)
                              }
                            }}
                          >
                            <TableCell className="retro text-xs font-medium">
                              <span className="mr-1" aria-hidden="true">
                                {expanded.has(job.name) ? "▾" : "▸"}
                              </span>
                              {job.name}
                            </TableCell>
                            <TableCell>
                              <StatusBadge status={job.status} />
                            </TableCell>
                            <TableCell className="retro text-xs">
                              {job.duration
                                ? `${Number(job.duration.seconds ?? 0n)}s`
                                : "—"}
                            </TableCell>
                          </TableRow>
                          {expanded.has(job.name) && job.steps.length > 0 && (
                            <StepRows
                              key={`${job.name}-steps`}
                              steps={job.steps}
                            />
                          )}
                        </>
                      ))
                    ) : (
                      <TableRow>
                        <TableCell
                          className="retro text-xs text-muted-foreground"
                          colSpan={3}
                        >
                          {loading ? (
                            <span className="flex items-center gap-2">
                              <Spinner label="Loading jobs" /> Loading…
                            </span>
                          ) : (
                            "No jobs yet"
                          )}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Logs tab */}
          <TabsContent value="logs">
            {!hasLogs && !running ? (
              <Card>
                <CardContent className="py-12">
                  <div className="text-center space-y-4">
                    <div className="text-4xl" aria-hidden="true">
                      📟
                    </div>
                    <p className="retro text-sm text-muted-foreground">
                      No logs available yet
                    </p>
                    <p className="retro text-xs text-muted-foreground">
                      Logs appear here during live pipeline runs. Re-run the
                      pipeline to stream logs in real time.
                    </p>
                    {data && data.jobs.length > 0 && (
                      <Button
                        size="sm"
                        onClick={() => void handleStartStream()}
                        disabled={running}
                      >
                        {running ? (
                          <span className="flex items-center gap-2">
                            <Spinner label="Streaming" /> Streaming…
                          </span>
                        ) : (
                          "▶️ Re-run & Stream"
                        )}
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            ) : (
              <LogViewer
                entries={logStream.entries}
                jobNames={logStream.jobNames}
                stepNames={logStream.stepNames}
                streaming={logStream.streaming}
              />
            )}
          </TabsContent>

          {/* Configuration tab */}
          <TabsContent value="config">
            <Card>
              <CardHeader>
                <CardTitle>Pipeline Configuration</CardTitle>
              </CardHeader>
              <CardContent>
                <ScrollArea className="h-48">
                  <pre className="retro text-xs text-muted-foreground">
                    {`# Pipeline configuration for "${data?.pipelineName || id}" will appear here.`}
                  </pre>
                </ScrollArea>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </Layout>
  )
}
