import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError, STARTING_CASH, toApiError } from '../api/client'
import type {
  ClockState,
  Employee,
  FinanceStatement,
  GameState,
  HiringState,
  Office,
  OfficeOffer,
  Package,
  Transaction,
} from '../api/types'

const POLL_INTERVAL_MS = 1000

export interface ErrorEntry {
  err: ApiError
  sticky: boolean
}

interface LocalSelection {
  officeId: string
  cash: number
}

function isMissingRoute(err: unknown): boolean {
  return err instanceof ApiError && err.status === 404
}

export function useGame() {
  const [clock, setClock] = useState<ClockState | null>(null)
  const [state, setState] = useState<GameState | null>(null)
  const [office, setOffice] = useState<Office | null>(null)
  const [offices, setOffices] = useState<OfficeOffer[]>([])
  const [packages, setPackages] = useState<Package[]>([])
  const [employees, setEmployees] = useState<Employee[]>([])
  const [hiring, setHiring] = useState<HiringState | null>(null)
  const [finance, setFinance] = useState<FinanceStatement | null>(null)
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [error, setError] = useState<ErrorEntry | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [selection, setSelection] = useState<LocalSelection | null>(null)

  const clockRef = useRef<ClockState | null>(null)
  const selectionRef = useRef<LocalSelection | null>(null)

  const refresh = useCallback(async () => {
    const results = await Promise.allSettled([
      api.getClock(),
      api.getGame(),
      api.getOffice(),
      api.getOffices(),
      api.getPackages(),
      api.getEmployees(),
      api.getFinance(),
      api.getTransactions(),
    ])

    const [clockResult, gameStateResult, officeResult, officesResult, packagesResult, employeesResult, financeResult, transactionsResult] =
      results

    if (clockResult.status === 'fulfilled') {
      clockRef.current = clockResult.value
      setClock(clockResult.value)
    }
    if (officeResult.status === 'fulfilled') {
      setOffice(officeResult.value)
    }
    if (officesResult.status === 'fulfilled') {
      setOffices(officesResult.value)
    }
    if (packagesResult.status === 'fulfilled') {
      setPackages(packagesResult.value)
    }
    if (employeesResult.status === 'fulfilled') {
      setEmployees(employeesResult.value.employees)
      setHiring(employeesResult.value.hiring)
    }
    if (financeResult.status === 'fulfilled') setFinance(financeResult.value)
    if (transactionsResult.status === 'fulfilled') setTransactions(transactionsResult.value)

    // The game state comes only from GET /game (SPEC 17.13: no client-side
    // synthesis). A missing route or a rejected read leaves the previous state in
    // place until the backend serves the endpoint again.
    if (gameStateResult.status === 'fulfilled' && gameStateResult.value !== null) {
      setState(gameStateResult.value)
      const officeId = gameStateResult.value.office?.id ?? null
      if (officeId) {
        const next: LocalSelection = {
          officeId,
          cash: gameStateResult.value.player.cash,
        }
        selectionRef.current = next
        setSelection(next)
      }
    }

    const hardFailures = results.filter((result) => {
      if (result.status !== 'rejected') return false
      if (isMissingRoute(result.reason)) return false
      return true
    })
    const firstFailure = hardFailures[0]
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
    async (officeId: string) => {
      try {
        const result = await api.selectOffice(officeId)
        const next: LocalSelection = {
          officeId: result.office_id,
          cash: result.cash_balance ?? selectionRef.current?.cash ?? STARTING_CASH,
        }
        selectionRef.current = next
        setSelection(next)
        if (result.state) {
          setState(result.state)
        }
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

  // SPEC 4.2: the run ends when the contract is not active and no new contract can be
  // afforded. A terminated contract that is still affordable frees the head-office slot
  // instead, so the UI offers re-selection — OfficeSelect reads that from
  // office.contract_status, which is the only place the backend reports it.
  const gameOver = state?.game.status === 'game_over'

  return {
    clock,
    state,
    office,
    offices,
    packages,
    employees,
    hiring,
    finance,
    transactions,
    error,
    loaded,
    selection,
    gameOver,
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
