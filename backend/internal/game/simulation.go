package game

import (
	"sync"
	"time"
)

// Simulation runs the background game loop: every interval of real time it advances
// the authoritative clock by the elapsed duration and processes every event that
// became due (SPEC 2.3: the simulation advances only while running, using
// monotonic elapsed runtime — no host wall-clock catch-up after downtime).
//
// The loop is owned by the application layer (started from main, stopped on
// shutdown). Persistence is decoupled: OnChange, when set, is invoked after any tick
// that changed state so the caller can snapshot.
type Simulation struct {
	state    *GameState
	interval time.Duration
	onChange func()
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

// StartSimulation launches the background loop on its own goroutine. The goroutine
// exits when Stop is called; Stop is idempotent and waits for the loop to finish, so
// a final snapshot afterwards never races the loop.
func (s *GameState) StartSimulation(interval time.Duration, onChange func()) *Simulation {
	sim := &Simulation{
		state:    s,
		interval: interval,
		onChange: onChange,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go sim.run()
	return sim
}

// run is the loop body: monotonic elapsed time between ticks drives Advance, and a
// changed result triggers OnChange (e.g. a persistence snapshot).
func (sim *Simulation) run() {
	defer close(sim.done)
	ticker := time.NewTicker(sim.interval)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case <-sim.stop:
			return
		case now := <-ticker.C:
			elapsed := now.Sub(last)
			last = now
			if sim.state.Tick(elapsed) && sim.onChange != nil {
				sim.onChange()
			}
		}
	}
}

// Stop signals the loop and blocks until it has exited. Safe to call more than once.
func (sim *Simulation) Stop() {
	sim.once.Do(func() { close(sim.stop) })
	<-sim.done
}
