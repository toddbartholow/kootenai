"""Test result models."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class TestStatus(str, Enum):
    """Status of a test execution."""

    PASSED = "passed"
    FAILED = "failed"
    SKIPPED = "skipped"
    ERROR = "error"


@dataclass
class TestResult:
    """Result of a single test."""

    name: str
    status: TestStatus
    duration: float
    message: str = ""
    details: dict[str, Any] = field(default_factory=dict)

    @property
    def passed(self) -> bool:
        return self.status == TestStatus.PASSED


@dataclass
class TestSuiteResult:
    """Results from a test suite."""

    name: str
    results: list[TestResult] = field(default_factory=list)
    duration: float = 0.0

    @property
    def total(self) -> int:
        return len(self.results)

    @property
    def passed(self) -> int:
        return sum(1 for r in self.results if r.status == TestStatus.PASSED)

    @property
    def failed(self) -> int:
        return sum(1 for r in self.results if r.status == TestStatus.FAILED)

    @property
    def skipped(self) -> int:
        return sum(1 for r in self.results if r.status == TestStatus.SKIPPED)

    @property
    def errors(self) -> int:
        return sum(1 for r in self.results if r.status == TestStatus.ERROR)

    @property
    def success(self) -> bool:
        return self.failed == 0 and self.errors == 0


@dataclass
class TestReport:
    """Complete test report with multiple suites."""

    suites: list[TestSuiteResult] = field(default_factory=list)
    duration: float = 0.0

    @property
    def total(self) -> int:
        return sum(s.total for s in self.suites)

    @property
    def passed(self) -> int:
        return sum(s.passed for s in self.suites)

    @property
    def failed(self) -> int:
        return sum(s.failed for s in self.suites)

    @property
    def skipped(self) -> int:
        return sum(s.skipped for s in self.suites)

    @property
    def errors(self) -> int:
        return sum(s.errors for s in self.suites)

    @property
    def success(self) -> bool:
        return all(s.success for s in self.suites)
