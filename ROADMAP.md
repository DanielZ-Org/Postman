# Delivery Office — Simplified Roadmap

This file describes the high-level direction only.

**GitHub Issues + GitHub Projects should be the working task tracker.**  
Do not turn this document into a second detailed issue system.

## Current state — Design / handover

Completed at specification level:

- Go backend selected.
- React + TypeScript frontend selected.
- REST/JSON API boundary agreed.
- SQLite selected for initial persistence.
- Local-first security principle agreed.
- Game clock/calendar rules defined.
- Player and initial traits defined.
- Small/large office contracts defined.
- Package core model defined.
- Employee core model defined.
- Walking/bike/car delivery capacities defined.
- Initial package-generation concept defined.
- Core finance/loan/rent/payroll model defined.
- Advanced events and weather deferred.

## M0 — Repository and collaboration setup

Goal: both developers can work independently against an agreed contract.

- Create/use GitHub organisation repository.
- Add `README.md`, `SPEC.md`, `ROADMAP.md`.
- Protect the API contract from undocumented breaking changes.
- Create GitHub Project.
- Convert implementation work into small GitHub Issues.
- Agree branch/PR workflow.
- Backend and frontend can still be developed and committed locally before pushing.

**Exit:** Daniel and Alexander can independently take an issue and know where the source of truth lives.

## M1 — Walking delivery vertical slice

Goal: prove the entire architecture with the smallest playable loop.

Implement only what is needed for:

**new game → office → packages → one walking employee → assignment → delivery → money → save/load → UI**

Backend:

- Go project skeleton.
- `/api/v1` API.
- game clock/state.
- player starting state.
- small office selection.
- basic packages.
- one walking employee/hiring path.
- delivery batch assignment.
- capacity = 10.
- delivery completion.
- revenue.
- wage accrual.
- basic finance statement.
- SQLite save/load.

Frontend:

- connect to backend.
- show game time.
- show cash.
- show warehouse usage/packages.
- show employee.
- assign a delivery batch.
- show delivery/result.
- show basic finance values.

**Exit:** a package can travel from warehouse to delivered state through the real backend and the UI can display the resulting authoritative state.

## M2 — Complete basic local delivery game

After M1 is stable:

- full clock controls: pause / 1x / 2x / 3x / skip;
- Monday-Saturday opening schedule;
- package generation;
- normal/express deadlines;
- late penalties;
- bicycle delivery;
- car/local delivery;
- multiple employees;
- Tuesday payroll;
- Friday rent;
- four-week loan interest;
- office capacity rules;
- player traits;
- game-over path for lost office/no replacement funds.

**Exit:** the basic local logistics management loop is playable over multiple game weeks.

## M3 — Balance and usability

Only after the core loop works:

- tune package rates/prices;
- employee speed traits;
- mood/retention;
- clearer finance reporting;
- package assignment priority;
- office upgrades;
- vehicle economics;
- UI polish.

## Future

Candidates, not commitments:

- weather;
- random events;
- training;
- large-package handling;
- far deliveries;
- drones;
- drone licences;
- aircraft;
- branch offices;
- decorations/equipment;
- richer technology/era progression;
- real-time push transport if REST polling becomes insufficient.

## Rule for taking work

Prefer one small GitHub Issue at a time.

An issue should state:

1. what changes;
2. what must not change;
3. API/schema impact;
4. acceptance criteria;
5. tests required.

Do not ask an AI coding agent to “build Delivery Office”. Give it one bounded issue with the relevant contract from `SPEC.md`.
