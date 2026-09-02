"""JSON report export."""

from __future__ import annotations

import json
from dataclasses import asdict
from datetime import datetime, timezone
from pathlib import Path

from ..models import PentestReport


def export_json(report: PentestReport, output_path: Path) -> Path:
    """Export report to JSON file."""
    data = asdict(report)
    data["generated_at"] = datetime.now(timezone.utc).isoformat()

    # Convert enums to strings
    _stringify_enums(data)

    filepath = output_path / "sectest-report.json"
    filepath.write_text(json.dumps(data, indent=2, default=str))
    return filepath


def _stringify_enums(obj: dict | list | str | int | float | None) -> None:
    """Recursively convert enum values to strings in a dict."""
    if isinstance(obj, dict):
        for key, value in obj.items():
            if hasattr(value, "value"):
                obj[key] = value.value
            elif isinstance(value, (dict, list)):
                _stringify_enums(value)
    elif isinstance(obj, list):
        for i, item in enumerate(obj):
            if hasattr(item, "value"):
                obj[i] = item.value
            elif isinstance(item, (dict, list)):
                _stringify_enums(item)
