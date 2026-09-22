"""Base test suite class."""

from __future__ import annotations
import time
from abc import ABC, abstractmethod
from typing import Callable, Coroutine, Optional

from ..clients.api import APIClient
from ..clients.database import DatabaseClient
from ..clients.nats import NATSClient
from ..clients.ssh import SSHClient
from ..config import LabtestConfig
from ..models.results import TestResult, TestStatus, TestSuiteResult


class BaseTestSuite(ABC):
    """Base class for test suites.

    Subclasses should:
    1. Set a `name` class attribute
    2. Implement the `run()` method
    3. Use `run_test()` to execute individual tests

    Usage:
        class MyTests(BaseTestSuite):
            name = "My Test Suite"

            async def run(self) -> TestSuiteResult:
                results = []
                results.append(await self.run_test("Test 1", self.test_one))
                results.append(await self.run_test("Test 2", self.test_two))
                return self.make_result(results)

            async def test_one(self):
                assert 1 + 1 == 2

            async def test_two(self):
                assert True
    """

    name: str = "Base Test Suite"

    def __init__(
        self,
        config: LabtestConfig,
        api: Optional[APIClient] = None,
        db: Optional[DatabaseClient] = None,
        nats: Optional[NATSClient] = None,
        ssh: Optional[SSHClient] = None,
    ):
        """Initialize test suite.

        Args:
            config: Test configuration
            api: API client (optional)
            db: Database client (optional)
            nats: NATS client (optional)
            ssh: SSH client (optional)
        """
        self.config = config
        self.api = api
        self.db = db
        self.nats = nats
        self.ssh = ssh
        self._start_time: float = 0

    @abstractmethod
    async def run(self) -> TestSuiteResult:
        """Run the test suite.

        Returns:
            Test suite result
        """
        pass

    async def run_test(
        self,
        name: str,
        test_func: Callable[[], Coroutine],
        skip_if: Optional[Callable[[], bool]] = None,
        skip_reason: str = "Precondition not met",
    ) -> TestResult:
        """Run a single test with error handling.

        Args:
            name: Test name
            test_func: Async test function
            skip_if: Optional function that returns True to skip test
            skip_reason: Reason for skipping

        Returns:
            Test result
        """
        # Check skip condition
        if skip_if and skip_if():
            return TestResult(
                name=name,
                status=TestStatus.SKIPPED,
                duration=0,
                message=skip_reason,
            )

        start = time.time()
        try:
            await test_func()
            return TestResult(
                name=name,
                status=TestStatus.PASSED,
                duration=time.time() - start,
                message="",
            )
        except AssertionError as e:
            return TestResult(
                name=name,
                status=TestStatus.FAILED,
                duration=time.time() - start,
                message=str(e) or "Assertion failed",
            )
        except Exception as e:
            return TestResult(
                name=name,
                status=TestStatus.ERROR,
                duration=time.time() - start,
                message=f"{type(e).__name__}: {e}",
            )

    def make_result(
        self,
        results: list[TestResult],
        start_time: Optional[float] = None,
    ) -> TestSuiteResult:
        """Create a test suite result.

        Args:
            results: List of test results
            start_time: Suite start time (uses _start_time if not provided)

        Returns:
            Test suite result
        """
        duration = time.time() - (start_time or self._start_time)
        return TestSuiteResult(
            name=self.name,
            results=results,
            duration=duration,
        )

    def require_api(self) -> bool:
        """Check if API client is available."""
        return self.api is not None

    def require_db(self) -> bool:
        """Check if database client is available."""
        return self.db is not None

    def require_nats(self) -> bool:
        """Check if NATS client is available."""
        return self.nats is not None

    def require_ssh(self) -> bool:
        """Check if SSH client is available."""
        return self.ssh is not None
