import { Link, useLocation } from "react-router"
import { Separator } from "@/components/ui/8bit/separator"
import { Button } from "@/components/ui/8bit/button"
import { useDarkMode } from "@/hooks/useDarkMode"

function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { dark, toggle } = useDarkMode()

  return (
    <div className="min-h-screen bg-background text-foreground transition-colors duration-300">
      {/* Skip to main content — accessibility */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:top-2 focus:left-2 focus:px-4 focus:py-2 focus:bg-primary focus:text-primary-foreground retro text-xs"
      >
        Skip to main content
      </a>

      {/* Header */}
      <header className="border-b border-border" role="banner">
        <div className="container mx-auto flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-6">
            <Link
              to="/"
              className="flex items-center gap-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary rounded"
              aria-label="seedee home"
            >
              <span className="retro text-lg font-bold tracking-tight">
                🌱 seedee
              </span>
            </Link>
            <nav
              className="hidden sm:flex items-center gap-4"
              aria-label="Main navigation"
            >
              <Link
                to="/"
                className="focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary rounded"
                aria-current={location.pathname === "/" ? "page" : undefined}
              >
                <span
                  className={`retro text-xs transition-colors duration-200 ${
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
            onClick={toggle}
            aria-label={dark ? "Switch to light mode" : "Switch to dark mode"}
          >
            {dark ? "☀️ Light" : "🌙 Dark"}
          </Button>
        </div>
      </header>
      <Separator />
      {/* Main content */}
      <main id="main-content" role="main">
        {children}
      </main>
    </div>
  )
}

export default Layout
