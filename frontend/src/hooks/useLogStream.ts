import { useCallback, useRef, useState } from "react";
import type { RunPipelineEvent } from "@/gen/seedee/v1/seedee_pb";
import { EventType, Status } from "@/gen/seedee/v1/seedee_pb";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface LogEntry {
  /** Monotonically increasing ID for keying / virtualizer. */
  id: number;
  /** ISO timestamp string. */
  timestamp: string;
  /** Job name that produced this log line. */
  jobName: string;
  /** Step name that produced this log line. */
  stepName: string;
  /** The actual text content (one line). */
  text: string;
  /** Whether this line came from stderr. */
  isStderr: boolean;
  /** Event type — LOG for output, others for lifecycle banners. */
  eventType: EventType;
}

export interface LogStreamState {
  /** All collected log entries. */
  entries: LogEntry[];
  /** Unique job names seen so far. */
  jobNames: string[];
  /** Unique step names seen so far (scoped to selected job, or all). */
  stepNames: string[];
  /** Whether the stream is still active. */
  streaming: boolean;
  /** Overall pipeline status (updated from lifecycle events). */
  pipelineStatus: Status;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const decoder = new TextDecoder();
let nextId = 1;

function eventToTimestamp(event: RunPipelineEvent): string {
  if (event.timestamp) {
    const ms =
      Number(event.timestamp.seconds ?? 0n) * 1000 +
      Math.floor((event.timestamp.nanos ?? 0) / 1_000_000);
    return new Date(ms).toISOString();
  }
  return new Date().toISOString();
}

function lifecycleLabel(event: RunPipelineEvent): string {
  switch (event.type) {
    case EventType.PIPELINE_STARTED:
      return "▶ Pipeline started";
    case EventType.PIPELINE_FINISHED:
      return `■ Pipeline finished (${Status[event.status]})`;
    case EventType.JOB_STARTED:
      return `▶ Job "${event.jobName}" started`;
    case EventType.JOB_FINISHED:
      return `■ Job "${event.jobName}" finished (${Status[event.status]})`;
    case EventType.JOB_SKIPPED:
      return `⏭ Job "${event.jobName}" skipped`;
    case EventType.STEP_STARTED:
      return `  ▶ Step "${event.stepName}" started`;
    case EventType.STEP_FINISHED: {
      const code = event.exitCode !== 0 ? ` exit=${event.exitCode}` : "";
      return `  ■ Step "${event.stepName}" finished (${Status[event.status]}${code})`;
    }
    default:
      return "";
  }
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * useLogStream accumulates RunPipelineEvent objects into structured log entries
 * suitable for rendering in the LogViewer.
 *
 * Call `push(event)` for every streamed event.
 * Call `complete()` when the stream ends.
 */
export function useLogStream() {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [jobNames, setJobNames] = useState<string[]>([]);
  const [stepNames, setStepNames] = useState<string[]>([]);
  const [streaming, setStreaming] = useState(true);
  const [pipelineStatus, setPipelineStatus] = useState<Status>(
    Status.UNSPECIFIED,
  );

  // Track seen names to avoid re-computing arrays on each push.
  const seenJobs = useRef(new Set<string>());
  const seenSteps = useRef(new Set<string>());

  const push = useCallback((event: RunPipelineEvent) => {
    // Track job / step names.
    if (event.jobName && !seenJobs.current.has(event.jobName)) {
      seenJobs.current.add(event.jobName);
      setJobNames((prev) => [...prev, event.jobName]);
    }
    if (event.stepName && !seenSteps.current.has(event.stepName)) {
      seenSteps.current.add(event.stepName);
      setStepNames((prev) => [...prev, event.stepName]);
    }

    // Update pipeline status on lifecycle events.
    if (
      event.type === EventType.PIPELINE_STARTED &&
      event.status === Status.UNSPECIFIED
    ) {
      setPipelineStatus(Status.RUNNING);
    }
    if (
      event.type === EventType.PIPELINE_FINISHED ||
      event.type === EventType.JOB_FINISHED ||
      event.type === EventType.STEP_FINISHED
    ) {
      if (event.type === EventType.PIPELINE_FINISHED) {
        setPipelineStatus(event.status);
      }
    }

    const ts = eventToTimestamp(event);
    const newEntries: LogEntry[] = [];

    if (event.type === EventType.STEP_LOG) {
      // Decode binary log data and split into lines.
      const raw = decoder.decode(event.logData);
      const lines = raw.split("\n");
      // If the last element is empty (trailing newline), drop it.
      if (lines.length > 0 && lines[lines.length - 1] === "") {
        lines.pop();
      }
      for (const line of lines) {
        newEntries.push({
          id: nextId++,
          timestamp: ts,
          jobName: event.jobName,
          stepName: event.stepName,
          text: line,
          isStderr: event.isStderr,
          eventType: event.type,
        });
      }
    } else {
      // Lifecycle event — generate a banner line.
      const label = lifecycleLabel(event);
      if (label) {
        newEntries.push({
          id: nextId++,
          timestamp: ts,
          jobName: event.jobName,
          stepName: event.stepName,
          text: label,
          isStderr: false,
          eventType: event.type,
        });
      }
    }

    if (newEntries.length > 0) {
      setEntries((prev) => [...prev, ...newEntries]);
    }
  }, []);

  const complete = useCallback(() => {
    setStreaming(false);
  }, []);

  const reset = useCallback(() => {
    setEntries([]);
    setJobNames([]);
    setStepNames([]);
    setStreaming(true);
    setPipelineStatus(Status.UNSPECIFIED);
    seenJobs.current.clear();
    seenSteps.current.clear();
  }, []);

  return {
    entries,
    jobNames,
    stepNames,
    streaming,
    pipelineStatus,
    push,
    complete,
    reset,
  };
}

// ---------------------------------------------------------------------------
// Filtering helper
// ---------------------------------------------------------------------------

export interface LogFilterOptions {
  /** Show only entries from this job (empty = all). */
  jobFilter: string;
  /** Show only entries from this step (empty = all). */
  stepFilter: string;
  /** Whether to show stderr lines. */
  showStderr: boolean;
  /** Text search query (case-insensitive). */
  searchQuery: string;
}

export function filterEntries(
  entries: LogEntry[],
  opts: LogFilterOptions,
): LogEntry[] {
  const { jobFilter, stepFilter, showStderr, searchQuery } = opts;
  const query = searchQuery.toLowerCase();

  return entries.filter((e) => {
    // Job filter — lifecycle events without a jobName always pass.
    if (jobFilter && e.jobName && e.jobName !== jobFilter) return false;
    // Step filter — lifecycle events without a stepName always pass.
    if (stepFilter && e.stepName && e.stepName !== stepFilter) return false;
    // Stderr toggle — only affects actual log lines.
    if (!showStderr && e.isStderr && e.eventType === EventType.STEP_LOG)
      return false;
    // Search.
    if (query && !e.text.toLowerCase().includes(query)) return false;
    return true;
  });
}
