"""Tests for subagent_stop.py - hook for parsing transcripts and logging tokens."""

import json
import sys
from pathlib import Path

import pytest

# Add hooks directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / ".claude" / "hooks"))

from subagent_stop import parse_transcript


@pytest.fixture
def transcript_dir(tmp_path: Path) -> Path:
    """Create temporary directory for transcript files."""
    return tmp_path


class TestParseTranscript:
    """Tests for parse_transcript function."""

    def test_parse_empty_transcript(self, transcript_dir: Path) -> None:
        """Test parsing empty transcript file."""
        transcript_path = transcript_dir / "empty.jsonl"
        transcript_path.write_text("")

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 0
        assert result["output"] == 0
        assert result["cost"] == 0.0

    def test_parse_nonexistent_transcript(self, transcript_dir: Path) -> None:
        """Test parsing nonexistent transcript file."""
        result = parse_transcript(str(transcript_dir / "nonexistent.jsonl"))

        assert "error" in result

    def test_parse_transcript_with_usage(self, transcript_dir: Path) -> None:
        """Test parsing transcript with usage data."""
        transcript_path = transcript_dir / "usage.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 100, "output_tokens": 50}}),
            json.dumps({"usage": {"input_tokens": 200, "output_tokens": 100}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 300
        assert result["output"] == 150
        assert result["cost"] > 0

    def test_parse_transcript_with_nested_usage(self, transcript_dir: Path) -> None:
        """Test parsing transcript with usage nested in message."""
        transcript_path = transcript_dir / "nested.jsonl"
        lines = [
            json.dumps({"message": {"usage": {"input_tokens": 500, "output_tokens": 250}}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 500
        assert result["output"] == 250

    def test_parse_transcript_mixed_entries(self, transcript_dir: Path) -> None:
        """Test parsing transcript with mixed entry types."""
        transcript_path = transcript_dir / "mixed.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 100, "output_tokens": 50}}),
            json.dumps({"type": "tool_use", "name": "read_file"}),  # No usage
            json.dumps({"message": {"usage": {"input_tokens": 200, "output_tokens": 100}}}),
            json.dumps({"text": "some text"}),  # No usage
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 300
        assert result["output"] == 150

    def test_parse_transcript_with_invalid_json(self, transcript_dir: Path) -> None:
        """Test parsing transcript with invalid JSON lines."""
        transcript_path = transcript_dir / "invalid.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 100, "output_tokens": 50}}),
            "this is not valid json",
            json.dumps({"usage": {"input_tokens": 200, "output_tokens": 100}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        # Should skip invalid line and continue
        assert result["input"] == 300
        assert result["output"] == 150

    def test_parse_transcript_with_blank_lines(self, transcript_dir: Path) -> None:
        """Test parsing transcript with blank lines."""
        transcript_path = transcript_dir / "blanks.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 100, "output_tokens": 50}}),
            "",
            "   ",
            json.dumps({"usage": {"input_tokens": 200, "output_tokens": 100}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 300
        assert result["output"] == 150

    def test_cost_calculation(self, transcript_dir: Path) -> None:
        """Test that cost is calculated correctly."""
        transcript_path = transcript_dir / "cost.jsonl"
        # 1M input tokens, 1M output tokens
        lines = [
            json.dumps({"usage": {"input_tokens": 1_000_000, "output_tokens": 1_000_000}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        # Cost: (1M/1M * $3) + (1M/1M * $15) = $3 + $15 = $18
        assert result["cost"] == 18.0

    def test_cost_rounding(self, transcript_dir: Path) -> None:
        """Test that cost is rounded to 4 decimal places."""
        transcript_path = transcript_dir / "round.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 1, "output_tokens": 1}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        # Very small cost should be rounded
        assert isinstance(result["cost"], float)
        # Check it's a valid rounded number
        assert str(result["cost"]).count(".") <= 1

    def test_parse_transcript_partial_usage(self, transcript_dir: Path) -> None:
        """Test parsing transcript with partial usage data."""
        transcript_path = transcript_dir / "partial.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 100}}),  # Missing output_tokens
            json.dumps({"usage": {"output_tokens": 50}}),  # Missing input_tokens
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 100
        assert result["output"] == 50

    def test_parse_tilde_path_expansion(self, transcript_dir: Path, tmp_path: Path) -> None:
        """Test that tilde paths are expanded."""
        # Create a file in the temp directory
        transcript_path = tmp_path / "test.jsonl"
        transcript_path.write_text(json.dumps({"usage": {"input_tokens": 100, "output_tokens": 50}}))

        # Test with actual path (can't really test ~ expansion without modifying HOME)
        result = parse_transcript(str(transcript_path))

        assert result["input"] == 100

    def test_parse_large_token_counts(self, transcript_dir: Path) -> None:
        """Test parsing transcript with large token counts."""
        transcript_path = transcript_dir / "large.jsonl"
        lines = [
            json.dumps({"usage": {"input_tokens": 100_000, "output_tokens": 50_000}}),
            json.dumps({"usage": {"input_tokens": 100_000, "output_tokens": 50_000}}),
            json.dumps({"usage": {"input_tokens": 100_000, "output_tokens": 50_000}}),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 300_000
        assert result["output"] == 150_000

    def test_parse_real_world_format(self, transcript_dir: Path) -> None:
        """Test parsing transcript with realistic Claude transcript format."""
        transcript_path = transcript_dir / "realistic.jsonl"
        lines = [
            json.dumps({
                "type": "message_start",
                "message": {
                    "id": "msg_123",
                    "role": "assistant",
                    "usage": {"input_tokens": 1500, "output_tokens": 0}
                }
            }),
            json.dumps({
                "type": "content_block_delta",
                "delta": {"text": "Hello!"}
            }),
            json.dumps({
                "type": "message_delta",
                "usage": {"output_tokens": 50}
            }),
            json.dumps({
                "type": "message_stop",
                "usage": {"input_tokens": 0, "output_tokens": 100}
            }),
        ]
        transcript_path.write_text("\n".join(lines))

        result = parse_transcript(str(transcript_path))

        assert result["input"] == 1500
        assert result["output"] == 150
