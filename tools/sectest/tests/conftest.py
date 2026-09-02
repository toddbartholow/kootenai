"""Shared fixtures for sectest unit tests."""

from __future__ import annotations

import pytest

from sectest.config import SectestConfig


@pytest.fixture
def config() -> SectestConfig:
    """Return a SectestConfig with safe defaults for testing."""
    return SectestConfig(
        target_host="127.0.0.1",
        target_port=8080,
        target_scheme="http",
        docker_bin="docker",
        output_dir="/tmp/sectest-test-output",
        verbose=False,
        scan_timeout=60,
    )
