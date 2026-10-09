"""Data models for security test results."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class Severity(str, Enum):
    CRITICAL = "critical"
    HIGH = "high"
    MEDIUM = "medium"
    LOW = "low"
    INFO = "info"


class ScanStatus(str, Enum):
    PASSED = "passed"
    FINDINGS = "findings"
    ERROR = "error"
    SKIPPED = "skipped"


@dataclass
class Finding:
    """A single security finding from any scanner."""

    title: str
    severity: Severity
    scanner: str
    description: str = ""
    evidence: str = ""
    remediation: str = ""
    reference: str = ""
    metadata: dict[str, Any] = field(default_factory=dict)

    @property
    def severity_rank(self) -> int:
        return {
            Severity.CRITICAL: 0,
            Severity.HIGH: 1,
            Severity.MEDIUM: 2,
            Severity.LOW: 3,
            Severity.INFO: 4,
        }.get(self.severity, 5)


@dataclass
class ScanResult:
    """Result from a single scanner."""

    scanner: str
    status: ScanStatus
    duration: float = 0.0
    findings: list[Finding] = field(default_factory=list)
    raw_output: str = ""
    error: str = ""

    @property
    def finding_counts(self) -> dict[str, int]:
        counts: dict[str, int] = {}
        for f in self.findings:
            counts[f.severity.value] = counts.get(f.severity.value, 0) + 1
        return counts


@dataclass
class PentestReport:
    """Aggregated report from all scanners."""

    target: str
    scan_results: list[ScanResult] = field(default_factory=list)
    duration: float = 0.0

    @property
    def all_findings(self) -> list[Finding]:
        findings = []
        for sr in self.scan_results:
            findings.extend(sr.findings)
        return sorted(findings, key=lambda f: f.severity_rank)

    @property
    def finding_counts(self) -> dict[str, int]:
        counts: dict[str, int] = {}
        for f in self.all_findings:
            counts[f.severity.value] = counts.get(f.severity.value, 0) + 1
        return counts

    @property
    def has_critical(self) -> bool:
        return any(f.severity == Severity.CRITICAL for f in self.all_findings)

    @property
    def has_high(self) -> bool:
        return any(f.severity == Severity.HIGH for f in self.all_findings)
