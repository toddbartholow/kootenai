"""Health and connectivity tests."""

from __future__ import annotations
import time

from .base import BaseTestSuite
from ..models.results import TestResult, TestSuiteResult


class HealthTestSuite(BaseTestSuite):
    """Quick health and connectivity tests.

    Tests API, database, and NATS connectivity.
    These are fast tests suitable for smoke testing.
    """

    name = "Health & Connectivity"

    async def run(self) -> TestSuiteResult:
        """Run all health tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # API tests
        results.append(
            await self.run_test(
                "API Health Check",
                self.test_api_health,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "API Ready Check",
                self.test_api_ready,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "API Version",
                self.test_api_version,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        # Database tests
        results.append(
            await self.run_test(
                "Database Connectivity",
                self.test_database_ping,
                skip_if=lambda: not self.require_db(),
                skip_reason="Database client not configured",
            )
        )

        results.append(
            await self.run_test(
                "Database Tables",
                self.test_database_tables,
                skip_if=lambda: not self.require_db(),
                skip_reason="Database client not configured",
            )
        )

        # NATS tests
        results.append(
            await self.run_test(
                "NATS Connectivity",
                self.test_nats_connection,
                skip_if=lambda: not self.require_nats(),
                skip_reason="NATS client not configured",
            )
        )

        results.append(
            await self.run_test(
                "NATS JetStream",
                self.test_nats_jetstream,
                skip_if=lambda: not self.require_nats(),
                skip_reason="NATS client not configured",
            )
        )

        return self.make_result(results)

    # API Tests

    async def test_api_health(self):
        """Test API health endpoint."""
        resp = await self.api.health()
        assert resp.status == "ok", f"Health check returned: {resp.status}"

    async def test_api_ready(self):
        """Test API readiness with component health."""
        resp = await self.api.ready()
        assert resp.status in ("ready", "degraded"), f"Ready check returned: {resp.status}"

        # Check individual components
        for name, check in resp.checks.items():
            if check.status == "unhealthy":
                raise AssertionError(f"Component {name} is unhealthy: {check.message}")

    async def test_api_version(self):
        """Test API version endpoint."""
        resp = await self.api.version()
        assert resp.version, "Version response missing version"
        assert resp.commit, "Version response missing commit"

    # Database Tests

    async def test_database_ping(self):
        """Test database connectivity."""
        result = await self.db.ping()
        assert result, "Database ping failed"

    async def test_database_tables(self):
        """Test that required database tables exist."""
        tables = await self.db.check_tables_exist()

        missing = [name for name, exists in tables.items() if not exists]
        if missing:
            raise AssertionError(f"Missing tables: {', '.join(missing)}")

    # NATS Tests

    async def test_nats_connection(self):
        """Test NATS connectivity."""
        connected = await self.nats.is_connected()
        assert connected, "NATS is not connected"

        # Test round-trip
        ping_ok = await self.nats.ping()
        assert ping_ok, "NATS ping failed"

    async def test_nats_jetstream(self):
        """Test NATS JetStream is configured."""
        stream_info = await self.nats.get_stream_info("LABS")
        assert stream_info is not None, "LABS stream not found"
        assert stream_info.get("name") == "LABS", "Stream name mismatch"


class QuickHealthSuite(BaseTestSuite):
    """Minimal health checks for fast diagnostics.

    Only checks basic connectivity, no detailed verification.
    """

    name = "Quick Health Check"

    async def run(self) -> TestSuiteResult:
        """Run quick health checks."""
        self._start_time = time.time()
        results: list[TestResult] = []

        results.append(
            await self.run_test(
                "API Reachable",
                self.test_api_reachable,
                skip_if=lambda: not self.require_api(),
            )
        )

        results.append(
            await self.run_test(
                "Database Reachable",
                self.test_db_reachable,
                skip_if=lambda: not self.require_db(),
            )
        )

        results.append(
            await self.run_test(
                "NATS Reachable",
                self.test_nats_reachable,
                skip_if=lambda: not self.require_nats(),
            )
        )

        return self.make_result(results)

    async def test_api_reachable(self):
        """Check API is reachable."""
        await self.api.health()

    async def test_db_reachable(self):
        """Check database is reachable."""
        await self.db.ping()

    async def test_nats_reachable(self):
        """Check NATS is reachable."""
        assert await self.nats.is_connected()
