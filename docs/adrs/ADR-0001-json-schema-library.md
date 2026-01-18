# ADR-0001: JSON Schema Validation Library

**Status:** Accepted
**Date:** 2026-01-18

## Applies To
- [Spec: Tasker Go Port](../specs/tasker-go-port.md)

## Context
The Tasker Go port requires JSON schema validation to validate state files, task definitions, capability maps, and other artifacts. The Python implementation uses the `jsonschema` library which supports JSON Schema draft-04 through draft-07.

We need a Go library that:
1. Supports the same JSON Schema drafts as Python
2. Is actively maintained
3. Has a clean API for embedding schemas

## Decision
Use `github.com/santhosh-tekuri/jsonschema` for JSON schema validation.

## Alternatives Considered

| Alternative | Pros | Cons | Why Not Chosen |
|-------------|------|------|----------------|
| xeipuuv/gojsonschema | Popular, simple API | Less complete spec coverage, maintenance uncertain | Spec compliance is important for compatibility |
| qri-io/jsonschema | Modern, draft-07 support | Less mature, smaller community | santhosh-tekuri more established |

## Consequences
- Full JSON Schema spec compliance (draft-04 through 2020-12)
- Schemas can be compiled once and reused
- Schema loading from filesystem or embedded
- May require more setup code than simpler libraries
- Active maintenance and good documentation

## Related
- Supersedes: (none)
- Related ADRs: (none)
