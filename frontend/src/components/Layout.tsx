import { Link, useLocation } from "react-router"
import { useEffect, useState } from "react"
import { Separator } from "@/components/ui/8bit/separator"
import { Button } from "@/components/ui/8bit/button"

function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const [dark, setDark] = useState(() => {
    if (typeof window !== "undefined") {
      return document.documentElement.classList.contains("dark")
    }
    return false
  })

  useEffect(() => {
    if (dark) {
      document.documentElement.classList.add("dark")
    } else {
      document.documentElement.classList.remove("dark")
    }
  }, [dark])

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Header */}
      <header className="border-b border-border">
        <div className="container mx-auto flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-6">
            <Link to="/" className="flex items-center gap-2">
              <span className="retro text-lg font-bold tracking-tight">
                🌱 seedee
              </span>
            </Link>
            <nav className="hidden sm:flex items-center gap-4">
              <Link to="/">
                <span
                  className={`retro text-xs ${
                    location.pathname === "/"
                      ? "text-foreground"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  Dashboard
                </span>
              </Link>
            </nav>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setDark((d) => !d)}
          >
            {dark ? "☀️ Light" : "🌙 Dark"}
          </Button>
        </div>
      </header>
      <Separator />
      {/* Main content */}
      <main>{children}</main>
    </div>
  )
}

export default Layout
