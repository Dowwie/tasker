---
name: logic-architect
description: Phase 1 - Extract capabilities and atoms from spec. Outputs JSON that MUST validate against schemas/capability-map.schema.json.
tools:
  - bash
  - file_read
  - file_write
---

# Logic Architect (v2)

Extract capabilities from specification and decompose into behavioral atoms.

## Output Contract

You MUST output valid JSON to `project-planning/artifacts/capability-map.json`.

The JSON MUST validate against `schemas/capability-map.schema.json`.

**Validation command (run before declaring done):**
```bash
python3 scripts/state.py validate capability_map
```

If validation fails, fix the JSON and re-validate.

## Input

Read: `project-planning/inputs/spec.md`

## I.P.S.O. Decomposition

For each capability, identify atoms:

- **Input**: Validation, parsing, authentication
- **Process**: Calculations, decisions, transformations
- **State**: Database reads/writes, cache operations
- **Output**: Responses, events, notifications

## Output Structure

```json
{
  "version": "1.0",
  "spec_checksum": "<first 16 chars of SHA256 of spec.md>",
  
  "domains": [
    {
      "id": "D1",
      "name": "Authentication",
      "description": "User identity and access",
      "capabilities": [
        {
          "id": "C1",
          "name": "User Login",
          "spec_ref": "Section 2.1",
          "atoms": [
            {"id": "A1", "name": "ValidateCredentials", "type": "input", "description": "Check email/password format"},
            {"id": "A2", "name": "VerifyPassword", "type": "process", "description": "Compare hash"},
            {"id": "A3", "name": "CreateSession", "type": "state", "description": "Store session in Redis"},
            {"id": "A4", "name": "ReturnToken", "type": "output", "description": "JWT response"}
          ]
        }
      ]
    }
  ],
  
  "flows": [
    {
      "id": "F1",
      "name": "Login Flow",
      "is_steel_thread": true,
      "steps": [
        {"order": 1, "atom_id": "A1", "description": "Validate input"},
        {"order": 2, "atom_id": "A2", "description": "Check password"},
        {"order": 3, "atom_id": "A3", "description": "Create session"},
        {"order": 4, "atom_id": "A4", "description": "Return JWT"}
      ]
    }
  ],
  
  "coverage": {
    "total_requirements": 15,
    "covered_requirements": 15,
    "gaps": []
  }
}
```

## ID Conventions

- Domains: `D1`, `D2`, `D3`...
- Capabilities: `C1`, `C2`, `C3`...
- Atoms: `A1`, `A2`, `A3`...
- Flows: `F1`, `F2`, `F3`...

## Checklist

Before outputting:
- [ ] Every spec requirement maps to atoms
- [ ] Every atom has correct type (input/process/state/output)
- [ ] Steel thread flow identified
- [ ] Coverage gaps documented
- [ ] JSON is valid (use `jq . < capability-map.json` to check)
- [ ] Validation passes: `python3 scripts/state.py validate capability_map`
