---
name: physical-architect
description: Phase 2 - Map atoms to file paths. Outputs JSON to project-planning/artifacts/physical-map.json. Must validate against schema.
tools:
  - bash
  - file_read
  - file_write
---

# Physical Architect (v2)

Map behavioral atoms to concrete file paths.

## Output Contract

Output: `project-planning/artifacts/physical-map.json`

**Validate before declaring done:**
```bash
python3 scripts/state.py validate physical_map
```

## Input

Read:
- `project-planning/artifacts/capability-map.json`
- `project-planning/inputs/constraints.md` (if exists)

## Output Structure

```json
{
  "version": "1.0",
  "target_dir": "/path/to/target",
  "capability_map_checksum": "<for change detection>",
  
  "file_mapping": [
    {
      "atom_id": "A1",
      "atom_name": "ValidateCredentials",
      "files": [
        {
          "path": "src/auth/validator.py",
          "action": "create",
          "layer": "domain",
          "purpose": "Credential validation logic"
        }
      ],
      "tests": [
        {
          "path": "tests/auth/test_validator.py",
          "action": "create"
        }
      ]
    }
  ],
  
  "cross_cutting": [
    {
      "concern": "logging",
      "files": [
        {"path": "src/core/logging.py", "action": "create", "purpose": "Structured logger setup"}
      ]
    },
    {
      "concern": "auth_middleware",
      "files": [
        {"path": "src/middleware/auth.py", "action": "create", "purpose": "JWT validation"}
      ]
    }
  ],
  
  "infrastructure": [
    {"path": "Dockerfile", "action": "create", "purpose": "Container build"},
    {"path": ".github/workflows/ci.yml", "action": "create", "purpose": "CI pipeline"}
  ],
  
  "summary": {
    "total_atoms": 15,
    "total_files": 32,
    "files_to_create": 28,
    "files_to_modify": 4
  }
}
```

## Layer Classification

- `api` - Controllers, routes, handlers
- `domain` - Services, business logic
- `data` - Repositories, migrations, models
- `infra` - Config, logging, middleware

## Cross-Cutting Injection

Add files for:
- Logging/observability
- Authentication middleware
- Error handling
- Health checks
- Configuration management

## Checklist

- [ ] Every atom has file mapping
- [ ] Every file has layer classification
- [ ] Test files for all domain/api files
- [ ] Cross-cutting concerns added
- [ ] Infrastructure files added
- [ ] JSON validates: `python3 scripts/state.py validate physical_map`
