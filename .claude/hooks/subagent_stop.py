#!/usr/bin/env python3
"""
SubagentStop Hook - delegates to state.py for token logging.

This hook fires when a subagent completes, parses the transcript,
and calls state.py to log the token usage.
"""

import json
import sys
from pathlib import Path


def parse_transcript(transcript_path: str) -> dict:
    """Extract token usage from transcript JSONL."""
    path = Path(transcript_path).expanduser()
    
    if not path.exists():
        return {"error": f"Not found: {path}"}
    
    total_input = 0
    total_output = 0
    
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
                usage = entry.get("usage", {})
                if not usage and "message" in entry:
                    usage = entry.get("message", {}).get("usage", {})
                if usage:
                    total_input += usage.get("input_tokens", 0)
                    total_output += usage.get("output_tokens", 0)
            except json.JSONDecodeError:
                continue
    
    # Estimate cost (adjust rates as needed)
    cost = (total_input / 1_000_000) * 3.0 + (total_output / 1_000_000) * 15.0
    
    return {
        "input": total_input,
        "output": total_output,
        "cost": round(cost, 4)
    }


def main():
    try:
        payload = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        print(f"Invalid payload: {e}", file=sys.stderr)
        sys.exit(1)
    
    transcript_path = payload.get("transcript_path", "")
    session_id = payload.get("session_id", "unknown")
    
    if not transcript_path:
        print("No transcript_path", file=sys.stderr)
        sys.exit(1)
    
    usage = parse_transcript(transcript_path)
    
    if "error" in usage:
        print(usage["error"], file=sys.stderr)
        sys.exit(1)
    
    # Find state.py relative to this hook
    hook_dir = Path(__file__).resolve().parent
    state_script = hook_dir.parent.parent / "scripts" / "state.py"
    
    if state_script.exists():
        import subprocess
        result = subprocess.run([
            "python3", str(state_script), "log-tokens",
            session_id[:8],
            str(usage["input"]),
            str(usage["output"]),
            str(usage["cost"])
        ], capture_output=True, text=True)
        
        if result.returncode != 0:
            print(f"state.py error: {result.stderr}", file=sys.stderr)
    
    # Always print summary
    print(f"Session {session_id[:8]}: {usage['input'] + usage['output']:,} tokens, ${usage['cost']:.4f}")


if __name__ == "__main__":
    main()
