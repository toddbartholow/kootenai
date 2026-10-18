"""Tests for the DockerRunner class.

Mocks subprocess.run to avoid requiring Docker during unit tests.
Verifies command construction, timeout handling, and image management.
"""

from __future__ import annotations

import subprocess
from unittest.mock import MagicMock, patch

import pytest

from sectest.config import SectestConfig
from sectest.docker_runner import DockerRunner


@pytest.fixture
def runner(config: SectestConfig) -> DockerRunner:
    """DockerRunner with a test config."""
    return DockerRunner(config)


class TestIsAvailable:
    """Test Docker availability detection."""

    def test_available_when_docker_responds(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="24.0.0")
            assert runner.is_available() is True

    def test_unavailable_when_docker_fails(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=1, stdout="")
            assert runner.is_available() is False

    def test_unavailable_when_file_not_found(self, runner: DockerRunner) -> None:
        with patch("subprocess.run", side_effect=FileNotFoundError):
            assert runner.is_available() is False

    def test_unavailable_when_timeout(self, runner: DockerRunner) -> None:
        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired("docker", 10)):
            assert runner.is_available() is False


class TestPullImage:
    """Test image pull logic."""

    def test_skip_pull_if_image_exists(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            # First call: image inspect succeeds
            mock_run.return_value = MagicMock(returncode=0)
            result = runner.pull_image("test:latest")
            assert result is True
            # Should only call inspect, not pull
            assert mock_run.call_count == 1

    def test_pull_if_image_missing(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            # First call: inspect fails, second: pull succeeds
            mock_run.side_effect = [
                MagicMock(returncode=1),  # inspect
                MagicMock(returncode=0),  # pull
            ]
            result = runner.pull_image("test:latest")
            assert result is True
            assert mock_run.call_count == 2

    def test_pull_failure(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.side_effect = [
                MagicMock(returncode=1),  # inspect
                MagicMock(returncode=1),  # pull fails
            ]
            result = runner.pull_image("test:latest")
            assert result is False


class TestRun:
    """Test container execution."""

    def test_basic_run(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(
                returncode=0, stdout="output", stderr=""
            )
            code, stdout, stderr = runner.run("image:tag", ["cmd", "arg"])
            assert code == 0
            assert stdout == "output"

    def test_run_with_volumes(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="", stderr="")
            runner.run("image:tag", ["scan"], volumes={"/host": "/container"})
            cmd_list = mock_run.call_args[0][0]
            assert "-v" in cmd_list
            idx = cmd_list.index("-v")
            assert cmd_list[idx + 1] == "/host:/container"

    def test_run_with_env(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="", stderr="")
            runner.run("image:tag", ["scan"], env={"KEY": "VAL"})
            cmd_list = mock_run.call_args[0][0]
            assert "-e" in cmd_list
            idx = cmd_list.index("-e")
            assert cmd_list[idx + 1] == "KEY=VAL"

    def test_run_with_network(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="", stderr="")
            runner.run("image:tag", ["scan"], network="bridge")
            cmd_list = mock_run.call_args[0][0]
            assert "--network" in cmd_list
            idx = cmd_list.index("--network")
            assert cmd_list[idx + 1] == "bridge"

    def test_run_with_name(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="", stderr="")
            runner.run("image:tag", ["scan"], name="test-scan")
            cmd_list = mock_run.call_args[0][0]
            assert "--name" in cmd_list
            idx = cmd_list.index("--name")
            assert cmd_list[idx + 1] == "test-scan"

    def test_timeout_handling(self, runner: DockerRunner) -> None:
        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired("docker", 60)):
            code, stdout, stderr = runner.run("image:tag", ["long-scan"], timeout=60)
            assert code == -1
            assert "timed out" in stderr

    def test_default_network_from_config(self, runner: DockerRunner) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="", stderr="")
            runner.run("image:tag", ["scan"])
            cmd_list = mock_run.call_args[0][0]
            idx = cmd_list.index("--network")
            # config fixture sets docker_network="host"
            assert cmd_list[idx + 1] == "host"


class TestEnsureImages:
    """Test batch image pulling."""

    def test_ensure_multiple_images(self, runner: DockerRunner) -> None:
        with patch.object(runner, "pull_image") as mock_pull:
            mock_pull.side_effect = [True, False, True]
            results = runner.ensure_images(["a:1", "b:2", "c:3"])
            assert results == {"a:1": True, "b:2": False, "c:3": True}
            assert mock_pull.call_count == 3
