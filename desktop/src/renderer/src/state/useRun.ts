import { useCallback, useEffect, useRef, useState } from 'react'
import { envelopeError, envelopePayload } from '@shared/envelope'
import type { RunResult } from '@shared/types'
import { useStore, type RunOptions } from './store'

export interface RunState<T> {
  pending: boolean
  result: RunResult | null
  data: T | null
  error: string | null
  errorCode: string | null
  exitCode: number | null
  durationMs: number | null
  run: (options?: RunOptions) => Promise<RunResult>
  reset: () => void
  setResult: (result: RunResult | null) => void
}

/**
 * One feature run with typed payload extraction.
 * Bob reports failures inside a 200-shaped envelope, so `error` covers both a
 * transport failure and an `ok:false` envelope.
 */
export function useRun<T = unknown>(featureId: string): RunState<T> {
  const { run } = useStore()
  const [pending, setPending] = useState(false)
  const [result, setResult] = useState<RunResult | null>(null)
  const mounted = useRef(true)

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  const execute = useCallback(
    async (options?: RunOptions): Promise<RunResult> => {
      setPending(true)
      try {
        const outcome = await run(featureId, options)
        if (mounted.current) setResult(outcome)
        return outcome
      } catch (error) {
        if (mounted.current) setResult(null)
        throw error
      } finally {
        if (mounted.current) setPending(false)
      }
    },
    [featureId, run]
  )

  const envelopeFailure = envelopeError(result?.envelope ?? null)
  const transportError = result && !result.envelope ? (result.parseError || result.stderr.trim() || `exit ${result.exitCode}`) : null

  return {
    pending,
    result,
    data: envelopePayload<T>(result?.envelope ?? null),
    error: envelopeFailure?.message ?? transportError,
    errorCode: envelopeFailure?.code ?? null,
    exitCode: result?.exitCode ?? null,
    durationMs: result ? result.durationMs : null,
    run: execute,
    reset: () => setResult(null),
    setResult: (next) => setResult(next)
  }
}
