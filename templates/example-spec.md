# Project Specification Template

Copy this file to `$TARGET_DIR/.tasker/inputs/spec.md` and customize for your project.

---

# {{PROJECT_NAME}} - Specification

## Overview

{{Brief description of what you're building and why}}

**Target Directory:** `{{/path/to/your/project}}`

## Goals

1. {{Primary goal}}
2. {{Secondary goal}}
3. {{etc.}}

## Non-Goals (Out of Scope)

- {{What this project explicitly will NOT do}}
- {{Boundaries to prevent scope creep}}

## Architecture

### High-Level Design

{{Describe the overall architecture - layers, components, data flow}}

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Component  │────▶│  Component  │────▶│  Component  │
└─────────────┘     └─────────────┘     └─────────────┘
```

### Components

#### {{Component 1}}
- **Purpose:** {{what it does}}
- **Inputs:** {{what it receives}}
- **Outputs:** {{what it produces}}
- **Dependencies:** {{what it needs}}

#### {{Component 2}}
{{repeat as needed}}

## API Contract (if applicable)

### Endpoints

#### `{{METHOD}} {{/path}}`
- **Purpose:** {{what it does}}
- **Request:**
  ```json
  {{request schema}}
  ```
- **Response:**
  ```json
  {{response schema}}
  ```
- **Errors:** {{error conditions}}

## Data Models

### {{Model Name}}
```
{{field}}: {{type}} - {{description}}
{{field}}: {{type}} - {{description}}
```

## Integration Points

### External Services
| Service | Purpose | Protocol |
|---------|---------|----------|
| {{name}} | {{purpose}} | {{REST/gRPC/etc.}} |

### Internal Dependencies
| Module | Purpose | Interface |
|--------|---------|-----------|
| {{name}} | {{purpose}} | {{how to use}} |

## Non-Functional Requirements

### Performance
- {{e.g., Response time < 200ms for P95}}
- {{e.g., Support 1000 concurrent requests}}

### Reliability
- {{e.g., 99.9% uptime}}
- {{e.g., Graceful degradation when dependencies fail}}

### Security
- {{e.g., Authentication via API keys}}
- {{e.g., Rate limiting per client}}

### Observability
- {{e.g., Structured logging with correlation IDs}}
- {{e.g., Distributed tracing}}
- {{e.g., Health check endpoints}}

## Success Criteria

The project is complete when:
- [ ] {{Measurable criterion 1}}
- [ ] {{Measurable criterion 2}}
- [ ] {{Measurable criterion 3}}

## Open Questions

- {{Question that needs to be resolved}}
- {{Decision that needs to be made}}

## References

- {{Link to related docs}}
- {{Link to design discussions}}
