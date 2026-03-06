import { useState, useRef, useCallback, useEffect } from "react"
import { useNavigate } from "react-router"
import { create } from "@bufbuild/protobuf"
import { toast } from "sonner"
import { Button } from "@/components/ui/8bit/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/8bit/card"
import { Spinner } from "@/components/Spinner"
import { useRunPipeline } from "@/hooks/usePipelines"
import { PipelineDefinitionSchema } from "@/gen/seedee/v1/seedee_pb"

interface RunPipelineDialogProps {
  open: boolean
  onClose: () => void
}

/**
 * Simple YAML text parsing for seedee pipeline definition.
 * Parses a minimal subset: name, env, jobs with image, depends_on, env, and steps.
 */
function parseYamlToPipeline(yaml: string) {
  const lines = yaml.split("\n")
  let name = ""
  const env: Record<string, string> = {}
  const jobs: Record<
    string,
    {
      image: string
      dependsOn: string[]
      env: Record<string, string>
      steps: { name: string; run: string; env: Record<string, string> }[]
    }
  > = {}

  let context:
    | "root"
    | "env"
    | "jobs"
    | "job"
    | "job-env"
    | "job-steps"
    | "step"
    | "step-env" = "root"
  let currentJob = ""
  let currentStep = -1

  for (const rawLine of lines) {
    const line = rawLine.replace(/\r$/, "")
    if (line.trim() === "" || line.trim().startsWith("#")) continue

    const indent = line.length - line.trimStart().length
    const trimmed = line.trim()

    if (indent === 0) {
      if (trimmed.startsWith("name:")) {
        name = trimmed.slice(5).trim().replace(/^["']|["']$/g, "")
        context = "root"
      } else if (trimmed === "env:") {
        context = "env"
      } else if (trimmed === "jobs:") {
        context = "jobs"
      }
      continue
    }

    if (context === "env" && indent === 2) {
      const [k, ...v] = trimmed.split(":")
      if (k)
        env[k.trim()] = v
          .join(":")
          .trim()
          .replace(/^["']|["']$/g, "")
      continue
    }

    if (context === "jobs" && indent === 2 && trimmed.endsWith(":")) {
      currentJob = trimmed.slice(0, -1)
      jobs[currentJob] = { image: "", dependsOn: [], env: {}, steps: [] }
      context = "job"
      continue
    }

    if (context === "job" && indent === 4) {
      if (trimmed.startsWith("image:")) {
        jobs[currentJob].image = trimmed
          .slice(6)
          .trim()
          .replace(/^["']|["']$/g, "")
      } else if (trimmed === "depends_on:") {
        context = "job"
      } else if (trimmed.startsWith("- ") && !trimmed.includes(":")) {
        jobs[currentJob].dependsOn.push(
          trimmed
            .slice(2)
            .trim()
            .replace(/^["']|["']$/g, ""),
        )
      } else if (trimmed === "env:") {
        context = "job-env"
      } else if (trimmed === "steps:") {
        context = "job-steps"
        currentStep = -1
      }
      continue
    }

    if (context === "job-env" && indent === 6) {
      const [k, ...v] = trimmed.split(":")
      if (k)
        jobs[currentJob].env[k.trim()] = v
          .join(":")
          .trim()
          .replace(/^["']|["']$/g, "")
      continue
    }

    if (context === "job-env" && indent <= 4) {
      context = "job"
    }

    if (context === "job-steps" && indent === 6 && trimmed.startsWith("- name:")) {
      const stepName = trimmed
        .slice(7)
        .trim()
        .replace(/^["']|["']$/g, "")
      jobs[currentJob].steps.push({ name: stepName, run: "", env: {} })
      currentStep = jobs[currentJob].steps.length - 1
      context = "step"
      continue
    }

    if (context === "step" && indent === 8) {
      if (trimmed.startsWith("run:")) {
        const runVal = trimmed
          .slice(4)
          .trim()
          .replace(/^["']|["']$/g, "")
        if (runVal === "|" || runVal === ">") {
          jobs[currentJob].steps[currentStep].run = ""
        } else {
          jobs[currentJob].steps[currentStep].run = runVal
        }
      } else if (trimmed === "env:") {
        context = "step-env"
      } else if (!trimmed.startsWith("- ")) {
        const step = jobs[currentJob].steps[currentStep]
        if (step.run !== undefined) {
          step.run = step.run ? step.run + "\n" + trimmed : trimmed
        }
      }
      continue
    }

    if (context === "step-env" && indent === 10) {
      const [k, ...v] = trimmed.split(":")
      if (k)
        jobs[currentJob].steps[currentStep].env[k.trim()] = v
          .join(":")
          .trim()
          .replace(/^["']|["']$/g, "")
      continue
    }

    if (
      (context === "step" || context === "step-env") &&
      indent === 6 &&
      trimmed.startsWith("- name:")
    ) {
      const stepName = trimmed
        .slice(7)
        .trim()
        .replace(/^["']|["']$/g, "")
      jobs[currentJob].steps.push({ name: stepName, run: "", env: {} })
      currentStep = jobs[currentJob].steps.length - 1
      context = "step"
      continue
    }

    if (
      (context === "job" ||
        context === "job-env" ||
        context === "job-steps" ||
        context === "step" ||
        context === "step-env") &&
      indent === 2 &&
      trimmed.endsWith(":")
    ) {
      currentJob = trimmed.slice(0, -1)
      jobs[currentJob] = { image: "", dependsOn: [], env: {}, steps: [] }
      context = "job"
      continue
    }
  }

  return create(PipelineDefinitionSchema, {
    name: name || "untitled",
    env,
    jobs: Object.fromEntries(
      Object.entries(jobs).map(([k, v]) => [
        k,
        {
          $typeName: "seedee.v1.JobDefinition" as const,
          image: v.image,
          dependsOn: v.dependsOn,
          env: v.env,
          steps: v.steps.map((s) => ({
            $typeName: "seedee.v1.StepDefinition" as const,
            name: s.name,
            run: s.run,
            env: s.env,
          })),
        },
      ]),
    ),
  })
}

const EXAMPLE_YAML = `name: hello-world
jobs:
  build:
    image: alpine:latest
    steps:
      - name: greet
        run: echo "Hello from seedee!"
`

export function RunPipelineDialog({ open, onClose }: RunPipelineDialogProps) {
  const [yaml, setYaml] = useState(EXAMPLE_YAML)
  const [parseError, setParseError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const { run, running, error: runError } = useRunPipeline()

  // Focus trap: focus the dialog when opened.
  useEffect(() => {
    if (open && dialogRef.current) {
      dialogRef.current.focus()
    }
  }, [open])

  // Escape to close.
  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [open, onClose])

  const handleFileUpload = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0]
      if (!file) return
      const reader = new FileReader()
      reader.onload = (ev) => {
        const text = ev.target?.result
        if (typeof text === "string") {
          setYaml(text)
          setParseError(null)
        }
      }
      reader.readAsText(file)
    },
    [],
  )

  const handleRun = useCallback(async () => {
    try {
      setParseError(null)
      const pipeline = parseYamlToPipeline(yaml)
      let pipelineId = ""

      await run(pipeline, {
        onEvent: (event) => {
          if (event.pipelineId && !pipelineId) {
            pipelineId = event.pipelineId
          }
        },
        onComplete: () => {
          toast.success("Pipeline submitted successfully")
          if (pipelineId) {
            navigate(`/pipeline/${pipelineId}`)
          }
          onClose()
        },
        onError: (err) => {
          toast.error("Pipeline run failed", { description: err.message })
        },
      })

      if (pipelineId) {
        navigate(`/pipeline/${pipelineId}`)
        onClose()
      }
    } catch {
      setParseError("Failed to parse YAML. Please check the format.")
      toast.error("Invalid YAML", {
        description: "Could not parse the pipeline definition.",
      })
    }
  }, [yaml, run, navigate, onClose])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center animate-in fade-in duration-200"
      role="dialog"
      aria-modal="true"
      aria-label="Run Pipeline"
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 transition-opacity"
        onClick={onClose}
        aria-hidden="true"
      />
      {/* Dialog */}
      <div
        ref={dialogRef}
        tabIndex={-1}
        className="relative z-10 w-full max-w-2xl mx-4 animate-in fade-in slide-in-from-bottom-4 duration-300 outline-none"
      >
        <Card>
          <CardHeader>
            <CardTitle>🚀 Run Pipeline</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div>
                <label
                  htmlFor="pipeline-yaml"
                  className="retro text-xs text-muted-foreground block mb-2"
                >
                  Paste your .seedee.yml configuration:
                </label>
                <textarea
                  id="pipeline-yaml"
                  value={yaml}
                  onChange={(e) => {
                    setYaml(e.target.value)
                    setParseError(null)
                  }}
                  className="retro w-full h-48 p-3 text-xs bg-background border-2 border-foreground dark:border-ring font-mono resize-y focus:outline-none focus:ring-2 focus:ring-primary"
                  placeholder="Paste your pipeline YAML here..."
                  spellCheck={false}
                  aria-describedby={parseError ? "yaml-error" : undefined}
                />
              </div>

              <div className="flex items-center gap-2">
                <span className="retro text-xs text-muted-foreground">or</span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => fileInputRef.current?.click()}
                >
                  📁 Upload .seedee.yml
                </Button>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".yml,.yaml"
                  onChange={handleFileUpload}
                  className="hidden"
                  aria-label="Upload pipeline YAML file"
                />
              </div>

              {parseError && (
                <p
                  id="yaml-error"
                  className="retro text-xs text-red-500"
                  role="alert"
                >
                  {parseError}
                </p>
              )}
              {runError && (
                <p className="retro text-xs text-red-500" role="alert">
                  Error: {runError.message}
                </p>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onClose}
                  disabled={running}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  onClick={handleRun}
                  disabled={running || !yaml.trim()}
                  aria-label={running ? "Pipeline is running" : "Run pipeline"}
                >
                  {running ? (
                    <span className="flex items-center gap-2">
                      <Spinner label="Running pipeline" /> Running…
                    </span>
                  ) : (
                    "▶️ Run"
                  )}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
