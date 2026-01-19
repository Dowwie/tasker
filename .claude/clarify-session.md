# Discovery: Tasker Go Port
Started: 2026-01-18

## Scope Summary

**Goal:** Port Tasker from Python scripts/modules to a single Go binary executable. The agentic parts (skills, agents, prompts) remain intact, but bash script invocations will call the Go binary with a unified CLI interface.

**Non-goals:**
- No feature changes (pure port)
- No agent changes (skills/agents/prompts stay as-is)
- No UI/UX changes (CLI interface behavior identical)

**Done means:**
- Every Python script has a Go equivalent subcommand
- Existing test suite passes against Go binary
- All skills/agents work without modification

---

## Category Status

| Category | Status | Notes |
|----------|--------|-------|
| Core requirements | ✓ Complete | CLI structure, subcommands, exact compatibility (REQ-001, 004, 005) |
| Users & context | ✓ Complete | Agents only, machine callers (REQ-018, 020) |
| Integrations / boundaries | ✓ Complete | JSON files, schemas, shim scripts, skill interface (REQ-003, 009, 030-033) |
| Edge cases / failures | ✓ Complete | Error handling, recovery, concurrency (REQ-015, 019, 024) |
| Quality attributes | ✓ Complete | No perf targets, log levels (REQ-012, 028) |
| Existing patterns | ✓ Complete | Terraform reference, Go 1.21 (REQ-021, 022) |
| Preferences / constraints | ✓ Complete | Go, macOS+Linux, same repo (REQ-002, 022, 025) |

---

## Questions Asked

## Answers Received

## Emerging Requirements

---

### Round 1

**Questions:**
1. How should the Go binary CLI be structured?
2. What is the target deployment environment?

**Answers:**
1. Single binary + subcommands (like git or kubectl): `tasker <command> <subcommand> [args]`
2. macOS + Linux (cross-platform CLI tool)

**Requirements Discovered:**
- REQ-001: Single Go binary with subcommand structure
- REQ-002: Cross-platform builds for macOS and Linux (arm64, amd64)

---

### Round 2

**Questions:**
1. How should the Go binary handle JSON schema validation?
2. Should the Go binary maintain exact CLI argument compatibility?

**Answers:**
1. User asked: "What is the best choice in the Go ecosystem?" → Decision deferred to ADR phase. Options: santhosh-tekuri/jsonschema (full spec), xeipuuv/gojsonschema (popular).
2. Exact compatibility - same args, same output format, drop-in replacement

**Requirements Discovered:**
- REQ-003: Must use a Go JSON schema validation library (specific choice TBD)
- REQ-004: Exact CLI argument compatibility with Python scripts (drop-in replacement)
- REQ-005: Output format must match current Python script output exactly

---

### Round 3

**Questions:**
1. Should the TUI be ported to Go or remain Python?
2. How should we handle the existing test suite?

**Answers:**
1. Port to Go, use the best available TUI library (bubbletea/lipgloss is standard)
2. Port tests to Go

**Requirements Discovered:**
- REQ-006: TUI must be ported to Go using established library (bubbletea/lipgloss)
- REQ-007: Test suite must be ported to Go
- REQ-008: Tests must validate identical behavior to Python implementation

---

### Round 4

**Questions:**
1. Should Go binary use same JSON file format as Python?
2. How should the Go binary be distributed?

**Answers:**
1. Same JSON format - exact same file structure for backward compatibility
2. Both - GitHub releases with binaries + Homebrew tap

**Requirements Discovered:**
- REQ-009: JSON file formats (state.json, task files) must remain identical
- REQ-010: Distribution via GitHub releases (pre-built binaries)
- REQ-011: Distribution via Homebrew tap for macOS/Linux

---

### Round 5

**Questions:**
1. Any performance requirements for Go binary vs Python?
2. What's the migration strategy?

**Answers:**
1. No specific targets - just needs to work correctly
2. Big bang - replace all Python scripts at once when ready

**Requirements Discovered:**
- REQ-012: No specific performance targets (correctness over speed)
- REQ-013: Big bang migration strategy (all scripts replaced at once)
- REQ-014: Migration requires all tests passing before switch

---

### Round 6

**Questions:**
1. How should the Go binary handle errors?
2. Should Go binary support configuration file?

**Answers:**
1. Idiomatic Go error handling - proper exit codes, stderr, structured where appropriate
2. CLI + env only - no config file, match current Python pattern

**Requirements Discovered:**
- REQ-015: Error handling follows Go idioms (non-zero exit codes, errors to stderr)
- REQ-016: No configuration file - CLI arguments and environment variables only
- REQ-017: Exit codes must match current Python script behavior for compatibility

---

### Round 7

**Questions:**
1. Who are the users of this system?
2. How should binary handle concurrent access to state files?

**Answers:**
1. Agents only - Claude Code agents/skills call the binary (not direct human use)
2. File locking - implement file locking for safe concurrent access

**Requirements Discovered:**
- REQ-018: Primary users are Claude Code agents/skills (machine callers)
- REQ-019: Implement file locking for concurrent state file access
- REQ-020: Output optimized for machine parsing (agents), not human readability

---

### Round 8

**Questions:**
1. Any existing Go codebases or conventions to follow?
2. What Go version should be minimum supported?

**Answers:**
1. Reference Terraform (https://github.com/hashicorp/terraform) for Go patterns
2. Go 1.21+ as minimum version

**Requirements Discovered:**
- REQ-021: Follow Terraform project structure and Go idioms
- REQ-022: Minimum Go version 1.21
- REQ-023: Use standard Go project layout (cmd/, internal/, pkg/ as appropriate)

---

### Round 9

**Questions:**
1. How should binary handle corrupted or invalid state files?
2. Should Go project live in same repo or separate?

**Answers:**
1. Auto-recover - attempt to recover/reset state if possible
2. Same repo - Go code in subdirectory

**Requirements Discovered:**
- REQ-024: Implement graceful recovery from corrupted state files
- REQ-025: Go code lives in same repo (subdirectory, e.g., go/ or cmd/)
- REQ-026: Build system must support both Python (legacy) and Go during transition

---

### Round 10

**Questions:**
1. Should Go TUI support mouse interaction or keyboard-only?
2. Should the binary have logging/debug output capability?

**Answers:**
1. Keyboard only - match current TUI behavior
2. Log levels - support multiple log levels (debug, info, warn, error)

**Requirements Discovered:**
- REQ-027: TUI keyboard-only navigation (no mouse)
- REQ-028: Support multiple log levels via flag (--log-level debug|info|warn|error)
- REQ-029: Default log level is warn (match current quiet behavior)

---

### Round 11

**Questions:**
1. How should skills discover the Go binary?
2. When updating skills, should we use shims or direct updates?

**Answers:**
1. User asked about prevailing patterns. For project-bundled tools, relative path from project root is common.
2. Shim scripts - wrapper scripts that redirect to tasker binary

**Requirements Discovered:**
- REQ-030: Create shim scripts at old paths (scripts/*.py) that exec Go binary
- REQ-031: Shim scripts translate arguments to Go subcommand format
- REQ-032: Skills continue calling `python3 scripts/state.py` → shim → `tasker state`
- REQ-033: Go binary installed to ./bin/tasker or similar project-relative path

---

### Round 12

**Questions:**
1. Should shell scripts (ensure-git.sh, init-planning-dirs.sh, log-activity.sh) be ported to Go?

**Answers:**
1. Port all - convert shell scripts to Go subcommands too

**Requirements Discovered:**
- REQ-034: Port all shell scripts to Go subcommands
- REQ-035: Complete inventory of scripts to port:
  - Python: state.py, validate.py, bundle.py, archive.py, dashboard.py, status.py, evaluate.py, compliance-check.py, spec-review.py, spec-generate.py, spec-session.py, fsm-compiler.py, fsm-mermaid.py, fsm-validate.py
  - Shell: ensure-git.sh, init-planning-dirs.sh, log-activity.sh
  - TUI module: scripts/tui/ (entire package)

---

## Discovery Complete

All category goals have been met:
- ✓ Core requirements: Primary workflows enumerated
- ✓ Users & context: User roles identified (agents), access patterns clear
- ✓ Integrations / boundaries: External systems named, data flows mapped
- ✓ Edge cases / failures: Error handling defined, retry/recovery specified
- ✓ Quality attributes: Performance (none), logging levels specified
- ✓ Existing patterns: Terraform reference, Go conventions
- ✓ Preferences / constraints: Go 1.21+, macOS/Linux, same repo

**Total Requirements Discovered:** 35 (REQ-001 through REQ-035)

<promise>CLARIFIED</promise>
