"""Shared fixtures for labtest unit tests.

Provides mock objects and test data that avoid requiring live services
(PostgreSQL, SSH, NATS, API) during unit testing.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest

from labtest.config import LabtestConfig
from labtest.simulator.parser import Objective, ParsedTemplate, Trigger, VMSpec


# ---------------------------------------------------------------------------
# Configuration fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def config() -> LabtestConfig:
    """Return a LabtestConfig with safe defaults (no real connections)."""
    return LabtestConfig(
        api_base_url="http://localhost:8080",
        db_host="localhost",
        db_port=5432,
        db_name="testdb",
        db_user="testuser",
        db_password="testpass",
        nats_url="nats://localhost:4222",
        verbose=False,
    )


# ---------------------------------------------------------------------------
# Template / trigger fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def sample_trigger() -> Trigger:
    """A simple file_exists trigger."""
    return Trigger(
        type="file_exists",
        target="lab-vm",
        match={"path": "/home/student/report.txt"},
    )


@pytest.fixture
def sample_objective(sample_trigger: Trigger) -> Objective:
    """A single objective with one trigger."""
    return Objective(
        id="obj-1",
        description="Create the report file",
        points=10,
        triggers=[sample_trigger],
        hint="Use `touch /home/student/report.txt`",
    )


@pytest.fixture
def sample_parsed_template(sample_objective: Objective) -> ParsedTemplate:
    """A minimal parsed template with one VM and one objective."""
    return ParsedTemplate(
        name="Test Lab",
        vms=[VMSpec(name="lab-vm", template="ubuntu-22.04", ip="10.0.0.10")],
        objectives=[sample_objective],
        pass_threshold=70,
        total_points=10,
    )


# ---------------------------------------------------------------------------
# Mock SSH client
# ---------------------------------------------------------------------------

@pytest.fixture
def mock_ssh_conn() -> AsyncMock:
    """Mock asyncssh.SSHClientConnection that returns successful results."""
    conn = AsyncMock()
    result = MagicMock()
    result.exit_status = 0
    result.stdout = ""
    result.stderr = ""
    conn.run.return_value = result
    return conn
