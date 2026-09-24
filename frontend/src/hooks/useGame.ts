import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError, toApiError } from '../api/client'
import type {
  ClockState,
  Employee,
  FinanceStatement,
  GameState,
  HiringState,
  Office,
  Package,
  Transaction,
} from '../api/types'

const POLL_INTERVAL_MS = 1000

export interface ErrorEntry {
  err: ApiError
  sticky: boolean
}

export function useGame() {
  const [clock, setClock] = useState<ClockState | null>(null)
  const [state, setState] = useState<GameState | null>(null)
  const [offices, setOffices] = useState<Office[]>([])
  const [packages, setPackages] = useState<Package[]>([])
  const [employees, setEmployees] = useState<Employee[]>([])
  const [hiring, setHiring] = useState<HiringState | null>(null)
  const [finance, setFinance] = useState<FinanceStatement | null>(null)
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [error, setError] = useState<ErrorEntry | null>(null)
  const [loaded, setLoaded] = useState(false)

  const clockRef = useRef<ClockState | null>(null)

  const refresh = useCallback(async () => {
    const results = await Promise.allSettled([
      api.getClock(),
      api.getGame(),
      api.getOffices(),
      api.getPackages(),
      api.getEmployees(),
      api.getFinance(),
      api.getTransactions(),
    ])

    const [clockResult, gameStateResult, officesResult, packagesResult, employeesResult, financeResult, transactionsResult] =
      results

    if (clockResult.status === 'fulfilled') {
      clockRef.current = clockResult.value
      setClock(clockResult.value)
    }
    if (gameStateResult.status === 'fulfilled') setState(gameStateResult.value)
    if (officesResult.status === 'fulfilled') setOffices(officesResult.value)
    if (packagesResult.status === 'fulfilled') setPackages(packagesResult.value)
    if (employeesResult.status === 'fulfilled') {
      setEmployees(employeesResult.value.employees)
      setHiring(employeesResult.value.hiring)
    }
    if (financeResult.status === 'fulfilled') setFinance(financeResult.value)
    if (transactionsResult.status === 'fulfilled') setTransactions(transactionsResult.value)

    const firstFailure = results.find((result) => result.status === 'rejected')
    if (firstFailure && firstFailure.status === 'rejected') {
      setError({ err: toApiError(firstFailure.reason), sticky: false })
    } else {
      setError((prev) => (prev !== null && !prev.sticky ? null : prev))
    }
  }, [])

  const dismissError = useCallback(() => {
    setError(null)
  }, [])

  const runMutation = useCallback(
    async (fn: () => Promise<unknown>): Promise<boolean> => {
      try {
        await fn()
        setError(null)
        await refresh()
        return true
      } catch (err) {
        setError({ err: toApiError(err), sticky: true })
        return false
      }
    },
    [refresh],
  )

  const setSpeed = useCallback(
    async (speed: 1 | 2 | 3) => {
      await runMutation(async () => {
        await api.setSpeed(speed)
        if (clockRef.current?.paused) {
          await api.setPaused(false)
        }
      })
    },
    [runMutation],
  )

  const setPaused = useCallback(
    async (paused: boolean) => {
      await runMutation(() => api.setPaused(paused))
    },
    [runMutation],
  )

  const togglePause = useCallback(async () => {
    const paused = clockRef.current?.paused ?? false
    await runMutation(() => api.setPaused(!paused))
  }, [runMutation])

  const skipToNextOpening = useCallback(async () => {
    await runMutation(() => api.skipToNextOpening())
  }, [runMutation])

  const selectOffice = useCallback(
    (officeId: string) => runMutation(() => api.selectOffice(officeId)),
    [runMutation],
  )

  const hireEmployee = useCallback(() => runMutation(() => api.hireEmployee()), [runMutation])

  const assignDelivery = useCallback(
    (employeeId: string, packageCount: number) =>
      runMutation(() => api.assignDelivery(employeeId, packageCount)),
    [runMutation],
  )

  const resetGame = useCallback(() => runMutation(() => api.resetGame()), [runMutation])

  useEffect(() => {
    let cancelled = false

    const tick = async () => {
      if (cancelled) return
      await refresh()
    }

    void (async () => {
      await tick()
      if (!cancelled) setLoaded(true)
    })()

    const interval = window.setInterval(() => {
      void tick()
    }, POLL_INTERVAL_MS)

    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [refresh])

  return {
    clock,
    state,
    offices,
    packages,
    employees,
    hiring,
    finance,
    transactions,
    error,
    loaded,
    dismissError,
    refresh,
    setSpeed,
    setPaused,
    togglePause,
    skipToNextOpening,
    selectOffice,
    hireEmployee,
    assignDelivery,
    resetGame,
  }
}

export type GameApi = ReturnType<typeof useGame>
