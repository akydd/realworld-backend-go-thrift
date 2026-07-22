# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

RealWorld backend API (social blogging, similar to Medium.com) written in Go using Hexagonal Architecture (Ports & Adapters). See @arch/arch.md for up-to-date architecture details — always consult it before making changes.

## Commands

```bash
make lint        # Run golangci-lint on all packages
make int-tests   # Build, start test DB, run full API integration test suite, teardown
go build ./cmd/server   # Build the server binary
./server -env .env_test  # Run server with test environment
```

## Development guidelines

- Always consult @arch/arch.md for current project information before writing or modifying code.
- When writing a plan for a feature file, write the plan to `features/plans/{feature-name}-plan.md`, where `feature-name` is the prefix of the corresponding feature file (e.g., `features/7-get-profile.md` → `features/plans/7-get-profile-plan.md`).
- When implementing a plan: iterate on code changes until `make lint` reports no errors.
- As the last step when implementing a plan, update @arch/arch.md to reflect any changes.
- When planning any changes to the database, ensure that the schema remains in 4NF. If this can't be done, then bring this to my attention after you've prepared the plan.
