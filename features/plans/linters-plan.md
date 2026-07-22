# Plan: Linters

## Overview

Add `golangci-lint` to the project and expose it via a `make lint` Makefile target. No custom configuration is required — the default golangci-lint ruleset will be used.

---

## 1. Install golangci-lint

golangci-lint's maintainers explicitly advise **against** installing it via `go install` or `go get`, as its dependencies require special build conditions. The two supported approaches are:

**Option A — Install script (any OS):**
```sh
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

**Option B — Homebrew (macOS):**
```sh
brew install golangci-lint
```

Either method installs the `golangci-lint` binary into a location on `$PATH`. No project files are modified by the installation itself.

---

## 2. Makefile — add `lint` target

Add `lint` to the existing `Makefile` alongside the existing `int-tests` target:

```makefile
.PHONY: int-tests lint

lint:
	golangci-lint run ./...
```

`./...` runs the linter across all packages in the module. With no config file present, golangci-lint uses its built-in default linter set.

---

## Summary of Files Changed

| File | Change |
|------|--------|
| `Makefile` | Add `lint` to `.PHONY`; add `lint` target |

No new Go dependencies, no config file. golangci-lint is installed as a system tool outside the module.
