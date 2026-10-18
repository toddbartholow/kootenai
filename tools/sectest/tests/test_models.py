"""Tests for sectest data models.

Verifies severity ranking, finding counts, report aggregation,
and computed properties on ScanResult and PentestReport.
"""

from __future__ import annotations

import pytest

from sectest.models import (
    Finding,
    PentestReport,
    ScanResult,
    ScanStatus,
    Severity,
)


# ---------------------------------------------------------------------------
# Severity enum
# ---------------------------------------------------------------------------

class TestSeverity:
    """Test severity enum values."""

    def test_all_values(self) -> None:
        assert Severity.CRITICAL == "critical"
        assert Severity.HIGH == "high"
        assert Severity.MEDIUM == "medium"
        assert Severity.LOW == "low"
        assert Severity.INFO == "info"


# ---------------------------------------------------------------------------
# Finding
# ---------------------------------------------------------------------------

class TestFinding:
    """Test the Finding dataclass."""

    def test_severity_rank_ordering(self) -> None:
        """CRITICAL < HIGH < MEDIUM < LOW < INFO by rank value."""
        ranks = [
            Finding(title="c", severity=Severity.CRITICAL, scanner="test").severity_rank,
            Finding(title="h", severity=Severity.HIGH, scanner="test").severity_rank,
            Finding(title="m", severity=Severity.MEDIUM, scanner="test").severity_rank,
            Finding(title="l", severity=Severity.LOW, scanner="test").severity_rank,
            Finding(title="i", severity=Severity.INFO, scanner="test").severity_rank,
        ]
        assert ranks == sorted(ranks)

    def test_finding_default_fields(self) -> None:
        f = Finding(title="Test", severity=Severity.LOW, scanner="nuclei")
        assert f.description == ""
        assert f.evidence == ""
        assert f.remediation == ""
        assert f.reference == ""
        assert f.metadata == {}


# ---------------------------------------------------------------------------
# ScanResult
# ---------------------------------------------------------------------------

class TestScanResult:
    """Test the ScanResult dataclass."""

    def test_finding_counts(self) -> None:
        result = ScanResult(
            scanner="nuclei",
            status=ScanStatus.FINDINGS,
            findings=[
                Finding(title="a", severity=Severity.HIGH, scanner="nuclei"),
                Finding(title="b", severity=Severity.HIGH, scanner="nuclei"),
                Finding(title="c", severity=Severity.LOW, scanner="nuclei"),
            ],
        )
        counts = result.finding_counts
        assert counts["high"] == 2
        assert counts["low"] == 1
        assert "critical" not in counts

    def test_empty_findings(self) -> None:
        result = ScanResult(scanner="zap", status=ScanStatus.PASSED)
        assert result.finding_counts == {}
        assert result.findings == []

    def test_error_status(self) -> None:
        result = ScanResult(
            scanner="ssl", status=ScanStatus.ERROR, error="connection refused"
        )
        assert result.status == ScanStatus.ERROR
        assert result.error == "connection refused"


# ---------------------------------------------------------------------------
# PentestReport
# ---------------------------------------------------------------------------

class TestPentestReport:
    """Test the PentestReport aggregate dataclass."""

    @pytest.fixture
    def report_with_findings(self) -> PentestReport:
        return PentestReport(
            target="http://localhost:8080",
            scan_results=[
                ScanResult(
                    scanner="nuclei",
                    status=ScanStatus.FINDINGS,
                    findings=[
                        Finding(title="XSS", severity=Severity.HIGH, scanner="nuclei"),
                        Finding(title="Info Disclosure", severity=Severity.INFO, scanner="nuclei"),
                    ],
                ),
                ScanResult(
                    scanner="zap",
                    status=ScanStatus.FINDINGS,
                    findings=[
                        Finding(title="SQL Injection", severity=Severity.CRITICAL, scanner="zap"),
                        Finding(title="CSRF", severity=Severity.MEDIUM, scanner="zap"),
                    ],
                ),
            ],
        )

    def test_all_findings_sorted_by_severity(self, report_with_findings: PentestReport) -> None:
        findings = report_with_findings.all_findings
        assert len(findings) == 4
        # First should be CRITICAL, last should be INFO
        assert findings[0].severity == Severity.CRITICAL
        assert findings[-1].severity == Severity.INFO

    def test_finding_counts_aggregated(self, report_with_findings: PentestReport) -> None:
        counts = report_with_findings.finding_counts
        assert counts["critical"] == 1
        assert counts["high"] == 1
        assert counts["medium"] == 1
        assert counts["info"] == 1

    def test_has_critical(self, report_with_findings: PentestReport) -> None:
        assert report_with_findings.has_critical is True

    def test_has_high(self, report_with_findings: PentestReport) -> None:
        assert report_with_findings.has_high is True

    def test_empty_report(self) -> None:
        report = PentestReport(target="http://localhost")
        assert report.all_findings == []
        assert report.finding_counts == {}
        assert report.has_critical is False
        assert report.has_high is False

    def test_report_no_critical(self) -> None:
        report = PentestReport(
            target="http://localhost",
            scan_results=[
                ScanResult(
                    scanner="ssl",
                    status=ScanStatus.FINDINGS,
                    findings=[
                        Finding(title="Weak cipher", severity=Severity.LOW, scanner="ssl"),
                    ],
                ),
            ],
        )
        assert report.has_critical is False
        assert report.has_high is False
