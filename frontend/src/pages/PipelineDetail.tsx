import { useParams, Link } from "react-router"
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
import { usePipelineStatus } from "@/hooks/usePipelines"

export default function PipelineDetail() {
  const { id } = useParams<{ id: string }>()
  const { data, loading } = usePipelineStatus({
    pipelineId: id ?? "",
    enabled: !!id,
  })

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

        <Tabs defaultValue="runs">
          <TabsList>
            <TabsTrigger value="runs">Jobs</TabsTrigger>
            <TabsTrigger value="config">Configuration</TabsTrigger>
          </TabsList>

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
