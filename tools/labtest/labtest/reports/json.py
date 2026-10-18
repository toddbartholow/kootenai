"""JSON reporter for test results."""

from __future__ import annotations
import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from ..models.results import TestReport, TestResult, TestStatus, TestSuiteResult


class JSONReporter:
    """Reporter that outputs test results as JSON.

    Usage:
        reporter = JSONReporter()
        json_str = reporter.to_json(report)
        reporter.save(report, "results.json")
    """

    def _result_to_dict(self, result: TestResult) -> dict[str, Any]:
        """Convert test result to dict."""
        return {
            "name": result.name,
            "status": result.status.value,
            "duration": result.duration,
            "message": result.message,
            "details": result.details,
        }

    def _suite_to_dict(self, suite: TestSuiteResult) -> dict[str, Any]:
        """Convert test suite result to dict."""
        return {
            "name": suite.name,
            "duration": suite.duration,
            "total": suite.total,
            "passed": suite.passed,
            "failed": suite.failed,
            "skipped": suite.skipped,
            "errors": suite.errors,
            "success": suite.success,
            "results": [self._result_to_dict(r) for r in suite.results],
        }

    def _report_to_dict(self, report: TestReport) -> dict[str, Any]:
        """Convert full report to dict."""
        return {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "duration": report.duration,
            "total": report.total,
            "passed": report.passed,
            "failed": report.failed,
            "skipped": report.skipped,
            "errors": report.errors,
            "success": report.success,
            "suites": [self._suite_to_dict(s) for s in report.suites],
        }

    def to_json(self, report: TestReport, indent: int = 2) -> str:
        """Convert report to JSON string.

        Args:
            report: Test report to convert
            indent: JSON indentation level

        Returns:
            JSON string
        """
        return json.dumps(self._report_to_dict(report), indent=indent)

    def save(self, report: TestReport, path: str | Path) -> None:
        """Save report to JSON file.

        Args:
            report: Test report to save
            path: Output file path
        """
        path = Path(path)
        path.write_text(self.to_json(report))

    def suite_to_json(self, suite: TestSuiteResult, indent: int = 2) -> str:
        """Convert a single suite to JSON.

        Args:
            suite: Test suite result
            indent: JSON indentation level

        Returns:
            JSON string
        """
        return json.dumps(self._suite_to_dict(suite), indent=indent)
