import { Link } from "react-router"
import Layout from "@/components/Layout"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
} from "@/components/ui/8bit/card"

export default function NotFound() {
  return (
    <Layout>
      <div className="container mx-auto py-16 px-4 flex items-center justify-center">
        <Card>
          <CardContent className="py-12">
            <div className="text-center space-y-4">
              <div className="text-6xl">🔍</div>
              <h1 className="retro text-xl font-bold">404 — Not Found</h1>
              <p className="retro text-xs text-muted-foreground">
                The page or pipeline you&apos;re looking for doesn&apos;t exist.
              </p>
              <Link to="/">
                <Button size="sm" className="mt-4">
                  ← Back to Dashboard
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}
