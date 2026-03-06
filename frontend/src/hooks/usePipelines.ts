import { useCallback, useEffect, useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { ciClient } from "@/lib/api";
import type {
  CancelPipelineResponse,
  GetPipelineStatusResponse,
  ListPipelinesResponse,
  RunPipelineEvent,
} from "@/gen/seedee/v1/seedee_pb";
import {
  ListPipelinesRequestSchema,
  GetPipelineStatusRequestSchema,
  CancelPipelineRequestSchema,
  RunPipelineRequestSchema,
  PipelineDefinitionSchema,
  Status,
} from "@/gen/seedee/v1/seedee_pb";
import type { PipelineDefinition } from "@/gen/seedee/v1/seedee_pb";

// ---------------------------------------------------------------------------
// useListPipelines — polls ListPipelines at a configurable interval
// ---------------------------------------------------------------------------

interface UseListPipelinesOptions {
  /** Polling interval in milliseconds (default: 3000). */
  intervalMs?: number;
  /** Optional status filter. */
  statusFilter?: Status;
}

interface UseListPipelinesResult {
  data: ListPipelinesResponse | null;
  error: Error | null;
  loading: boolean;
}

export function useListPipelines(
  options: UseListPipelinesOptions = {},
): UseListPipelinesResult {
  const { intervalMs = 3000, statusFilter = Status.UNSPECIFIED } = options;
  const [data, setData] = useState<ListPipelinesResponse | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function fetchPipelines() {
      try {
        const req = create(ListPipelinesRequestSchema, {
          statusFilter,
        });
        const res = await ciClient.listPipelines(req);
        if (!cancelled) {
          setData(res);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void fetchPipelines();
    const id = setInterval(() => void fetchPipelines(), intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs, statusFilter]);

  return { data, error, loading };
}

// ---------------------------------------------------------------------------
// usePipelineStatus — polls GetPipelineStatus at a configurable interval
// ---------------------------------------------------------------------------

interface UsePipelineStatusOptions {
  /** Pipeline ID to query. */
  pipelineId: string;
  /** Polling interval in milliseconds (default: 2000). */
  intervalMs?: number;
  /** Whether polling is enabled (default: true). */
  enabled?: boolean;
}

interface UsePipelineStatusResult {
  data: GetPipelineStatusResponse | null;
  error: Error | null;
  loading: boolean;
}

export function usePipelineStatus(
  options: UsePipelineStatusOptions,
): UsePipelineStatusResult {
  const { pipelineId, intervalMs = 2000, enabled = true } = options;
  const [data, setData] = useState<GetPipelineStatusResponse | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!enabled || !pipelineId) {
      setLoading(false);
      return;
    }

    let cancelled = false;

    async function fetchStatus() {
      try {
        const req = create(GetPipelineStatusRequestSchema, { pipelineId });
        const res = await ciClient.getPipelineStatus(req);
        if (!cancelled) {
          setData(res);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void fetchStatus();
    const id = setInterval(() => void fetchStatus(), intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [pipelineId, intervalMs, enabled]);

  return { data, error, loading };
}

// ---------------------------------------------------------------------------
// useCancelPipeline — mutation to cancel a running pipeline
// ---------------------------------------------------------------------------

interface UseCancelPipelineResult {
  cancel: (pipelineId: string) => Promise<CancelPipelineResponse>;
  data: CancelPipelineResponse | null;
  error: Error | null;
  loading: boolean;
}

export function useCancelPipeline(): UseCancelPipelineResult {
  const [data, setData] = useState<CancelPipelineResponse | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(false);

  const cancel = useCallback(async (pipelineId: string) => {
    setLoading(true);
    setError(null);
    try {
      const req = create(CancelPipelineRequestSchema, { pipelineId });
      const res = await ciClient.cancelPipeline(req);
      setData(res);
      return res;
    } catch (err) {
      const wrapped =
        err instanceof Error ? err : new Error(String(err));
      setError(wrapped);
      throw wrapped;
    } finally {
      setLoading(false);
    }
  }, []);

  return { cancel, data, error, loading };
}

// ---------------------------------------------------------------------------
// useRunPipeline — streaming RPC with event callbacks
// ---------------------------------------------------------------------------

interface RunPipelineCallbacks {
  /** Called for every streamed event. */
  onEvent?: (event: RunPipelineEvent) => void;
  /** Called when the stream completes successfully. */
  onComplete?: () => void;
  /** Called when the stream errors. */
  onError?: (error: Error) => void;
}

interface UseRunPipelineResult {
  run: (
    pipeline: PipelineDefinition,
    callbacks?: RunPipelineCallbacks,
  ) => Promise<void>;
  abort: () => void;
  running: boolean;
  error: Error | null;
}

export function useRunPipeline(): UseRunPipelineResult {
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const abort = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  // Clean up on unmount.
  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  const run = useCallback(
    async (
      pipeline: PipelineDefinition,
      callbacks?: RunPipelineCallbacks,
    ) => {
      // Abort any in-flight stream.
      abortRef.current?.abort();

      const ac = new AbortController();
      abortRef.current = ac;

      setRunning(true);
      setError(null);

      try {
        const req = create(RunPipelineRequestSchema, {
          pipeline: create(PipelineDefinitionSchema, {
            name: pipeline.name,
            env: pipeline.env,
            jobs: pipeline.jobs,
          }),
        });

        const stream = ciClient.runPipeline(req, {
          signal: ac.signal,
        });

        for await (const event of stream) {
          if (ac.signal.aborted) break;
          callbacks?.onEvent?.(event);
        }

        if (!ac.signal.aborted) {
          callbacks?.onComplete?.();
        }
      } catch (err) {
        if (ac.signal.aborted) return;
        const wrapped =
          err instanceof Error ? err : new Error(String(err));
        setError(wrapped);
        callbacks?.onError?.(wrapped);
      } finally {
        setRunning(false);
      }
    },
    [],
  );

  return { run, abort, running, error };
}
