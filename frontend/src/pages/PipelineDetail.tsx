import { useParams, Link } from "react-router"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import { Badge } from "@/components/ui/8bit/badge"
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

export default function PipelineDetail() {
  const { id } = useParams<{ id: string }>()

  return (
    <div className="container mx-auto py-8 px-4">
      <div className="mb-6">
        <Link to="/">
          <Button variant="ghost" size="sm" className="mb-4">
            ← Back to Dashboard
          </Button>
        </Link>
        <div className="flex items-center gap-3">
          <h1 className="retro text-2xl font-bold tracking-tight">
            Pipeline: {id}
          </h1>
          <Badge variant="secondary">idle</Badge>
        </div>
      </div>

      <Separator className="mb-6" />

      <Tabs defaultValue="runs">
        <TabsList>
          <TabsTrigger value="runs">Runs</TabsTrigger>
          <TabsTrigger value="config">Configuration</TabsTrigger>
        </TabsList>

        <TabsContent value="runs">
          <Card>
            <CardHeader>
              <CardTitle>Recent Runs</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Run</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Duration</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell
                      className="text-muted-foreground"
                      colSpan={3}
                    >
                      No runs yet
                    </TableCell>
                  </TableRow>
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
                  {`# Pipeline configuration for "${id}" will appear here.`}
                </pre>
              </ScrollArea>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
