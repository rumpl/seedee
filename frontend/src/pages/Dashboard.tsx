import { Link } from "react-router"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import { Badge } from "@/components/ui/8bit/badge"

export default function Dashboard() {
  return (
    <div className="container mx-auto py-8 px-4">
      <div className="mb-8">
        <h1 className="retro text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="retro text-xs text-muted-foreground mt-2">
          Monitor your CI/CD pipelines
        </p>
      </div>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Example Pipeline</CardTitle>
              <Badge variant="secondary">idle</Badge>
            </div>
            <CardDescription>An example pipeline</CardDescription>
          </CardHeader>
          <CardContent>
            <Link to="/pipeline/example">
              <Button variant="outline" size="sm">
                View details
              </Button>
            </Link>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
