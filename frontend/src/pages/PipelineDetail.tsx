import { useParams, Link } from "react-router"
import { useCallback, useMemo, useState } from "react"
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
import { usePipelineStatus, useRunPipeline } from "@/hooks/usePipelines"
import { useLogStream } from "@/hooks/useLogStream"
import { create } from "@bufbuild/protobuf"
import {
  Status,
  PipelineDefinitionSchema,
} from "@/gen/seedee/v1/seedee_pb"
import type { RunPipelineEvent } from "@/gen/seedee/v1/seedee_pb"

export default function PipelineDetail() {
  const { id } = useParams<{ id: string }>()
  const { data, loading } = usePipelineStatus({
    pipelineId: id ?? "",
    enabled: !!id,
  })

  // --- Log stream state ---
  const logStream = useLogStream()
  const { run, running } = useRunPipeline()
  const [hasLogs, setHasLogs] = useState(false)

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
    // Build a minimal pipeline definition from the status data to re-run.
    // In a real app this would come from a stored definition; here we use the
    // status jobs to build a skeleton.
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
      onComplete: () => logStream.complete(),
      onError: () => logStream.complete(),
    })
  }, [data, run, logStream, handleEvent])

  // Determine if pipeline is/was running (show log viewer tab).
  const isRunning = data?.status === Status.RUNNING
  const showLogs = hasLogs || isRunning

  // Active tab — default to logs if we have them.
  const defaultTab = useMemo(
    () => (showLogs ? "logs" : "runs"),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [], // only set initial value
  )

  return (
    <Layout>
      <div className="container mx-auto py-8 px-4">
        <div className="mb-6">
          <Link to="/">
            <Button variant="ghost" size="sm" className="mb-4">
              ← Back to Dashboard
            </Button>
          </Link>
          <div className="flex items-center gap-3">
            <h1 className="retro text-2xl font-bold tracking-tight">
              Pipeline: {data?.pipelineName || id}
            </h1>
            {data && <StatusBadge status={data.status} />}
            {loading && !data && (
              <div className="h-5 w-20 bg-muted animate-pulse" />
            )}
          </div>
        </div>

        <Separator className="mb-6" />

        <Tabs defaultValue={defaultTab}>
          <TabsList>
            <TabsTrigger value="runs">Jobs</TabsTrigger>
            <TabsTrigger value="logs">
              📟 Logs
              {logStream.streaming && hasLogs && (
                <span className="ml-1 inline-block w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
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
                        <TableRow key={job.name}>
                          <TableCell className="retro text-xs font-medium">
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
                      ))
                    ) : (
                      <TableRow>
                        <TableCell
                          className="retro text-xs text-muted-foreground"
                          colSpan={3}
                        >
                          {loading ? "Loading..." : "No jobs yet"}
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
                    <div className="text-4xl">📟</div>
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
                        {running ? "⏳ Streaming..." : "▶️ Re-run & Stream"}
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
