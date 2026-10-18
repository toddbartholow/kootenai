"""Tests for SSH client shell-quoting safety.

Verifies that SSHClient methods use shlex.quote() on path and pattern
parameters to prevent shell injection through filenames or search patterns.

These tests mock the underlying SSH connection to avoid real network calls.
The assertions inspect the command strings passed to conn.run() to confirm
that arguments are properly quoted.
"""

from __future__ import annotations

import asyncio
import shlex
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from labtest.clients.ssh import CommandResult, SSHClient


@pytest.fixture
def ssh_client() -> SSHClient:
    """Create an SSHClient with a mocked connection."""
    client = SSHClient(
        host="10.0.0.1",
        username="testuser",
        password="testpass",
    )
    # Install a mock connection that records the commands
    mock_conn = AsyncMock()
    result = MagicMock()
    result.exit_status = 0
    result.stdout = "exists"
    result.stderr = ""
    mock_conn.run.return_value = result
    client._conn = mock_conn
    return client


def _get_last_command(client: SSHClient) -> str:
    """Extract the command string from the last conn.run() call."""
    call_args = client._conn.run.call_args
    return call_args[0][0]


class TestCommandResult:
    """Test the CommandResult dataclass."""

    def test_success_when_zero_exit(self) -> None:
        result = CommandResult(exit_code=0, stdout="ok", stderr="")
        assert result.success is True

    def test_failure_when_nonzero_exit(self) -> None:
        result = CommandResult(exit_code=1, stdout="", stderr="error")
        assert result.success is False

    def test_failure_when_negative_exit(self) -> None:
        result = CommandResult(exit_code=-1, stdout="", stderr="timeout")
        assert result.success is False


class TestFileExistsQuoting:
    """Verify file_exists() quotes the path argument."""

    async def test_simple_path(self, ssh_client: SSHClient) -> None:
        await ssh_client.file_exists("/home/user/file.txt")
        cmd = _get_last_command(ssh_client)
        assert "/home/user/file.txt" in cmd
        assert "test -e" in cmd

    @pytest.mark.parametrize(
        "dangerous_path",
        [
            "/tmp/$(rm -rf /)",
            "/tmp/; rm -rf /",
            "/tmp/`whoami`",
            "/home/user/file name with spaces.txt",
            "/home/user/file\twith\ttabs",
            "/tmp/path'with'quotes",
            '/tmp/path"with"double',
            "/tmp/$HOME/.ssh/authorized_keys",
            "/tmp/file|cat /etc/passwd",
        ],
    )
    async def test_dangerous_paths_are_quoted(
        self, ssh_client: SSHClient, dangerous_path: str
    ) -> None:
        """Shell metacharacters in paths must be quoted to prevent injection."""
        await ssh_client.file_exists(dangerous_path)
        cmd = _get_last_command(ssh_client)
        # The shlex-quoted version should appear in the command
        assert shlex.quote(dangerous_path) in cmd


class TestReadFileQuoting:
    """Verify read_file() quotes the path argument."""

    async def test_normal_path(self, ssh_client: SSHClient) -> None:
        await ssh_client.read_file("/etc/hostname")
        cmd = _get_last_command(ssh_client)
        assert "cat" in cmd
        assert "/etc/hostname" in cmd

    async def test_path_with_spaces_is_quoted(self, ssh_client: SSHClient) -> None:
        path = "/home/user/my file.txt"
        await ssh_client.read_file(path)
        cmd = _get_last_command(ssh_client)
        assert shlex.quote(path) in cmd

    async def test_path_with_shell_injection_is_quoted(self, ssh_client: SSHClient) -> None:
        path = "/tmp/$(cat /etc/shadow)"
        await ssh_client.read_file(path)
        cmd = _get_last_command(ssh_client)
        assert shlex.quote(path) in cmd


class TestWriteFileQuoting:
    """Verify write_file() quotes both content and path."""

    async def test_content_and_path_quoted(self, ssh_client: SSHClient) -> None:
        path = "/tmp/test; rm -rf /"
        content = "hello $(whoami)"
        await ssh_client.write_file(path, content)
        cmd = _get_last_command(ssh_client)
        assert shlex.quote(path) in cmd
        assert shlex.quote(content) in cmd


class TestMkdirQuoting:
    """Verify mkdir() quotes the path argument."""

    async def test_mkdir_with_parents(self, ssh_client: SSHClient) -> None:
        await ssh_client.mkdir("/tmp/new dir/sub")
        cmd = _get_last_command(ssh_client)
        assert "-p" in cmd
        assert shlex.quote("/tmp/new dir/sub") in cmd

    async def test_mkdir_without_parents(self, ssh_client: SSHClient) -> None:
        await ssh_client.mkdir("/tmp/simple", parents=False)
        cmd = _get_last_command(ssh_client)
        assert "-p" not in cmd
        assert shlex.quote("/tmp/simple") in cmd


class TestRemoveQuoting:
    """Verify remove() quotes the path argument."""

    async def test_remove_file(self, ssh_client: SSHClient) -> None:
        path = "/tmp/$(rm -rf /)"
        await ssh_client.remove(path)
        cmd = _get_last_command(ssh_client)
        assert shlex.quote(path) in cmd
        assert "-f" in cmd

    async def test_remove_recursive(self, ssh_client: SSHClient) -> None:
        await ssh_client.remove("/tmp/dir; evil", recursive=True)
        cmd = _get_last_command(ssh_client)
        assert "-rf" in cmd
        assert shlex.quote("/tmp/dir; evil") in cmd


class TestFileContainsQuoting:
    """Verify file_contains() quotes both path and pattern."""

    async def test_both_args_quoted(self, ssh_client: SSHClient) -> None:
        path = "/etc/passwd; echo hacked"
        pattern = "root.*$(cmd)"
        await ssh_client.file_contains(path, pattern)
        cmd = _get_last_command(ssh_client)
        assert shlex.quote(path) in cmd
        assert shlex.quote(pattern) in cmd
        assert "grep -q" in cmd


class TestRunTimeout:
    """Verify the run() method handles timeouts gracefully."""

    async def test_timeout_returns_negative_exit_code(self, ssh_client: SSHClient) -> None:
        """When a command times out, run() should return exit_code=-1."""
        ssh_client._conn.run.side_effect = asyncio.TimeoutError()
        # Override wait_for to raise immediately
        with patch("asyncio.wait_for", side_effect=asyncio.TimeoutError()):
            result = await ssh_client.run("sleep 999", timeout=1)
        assert result.exit_code == -1
        assert "timed out" in result.stderr

    async def test_exception_returns_error_result(self, ssh_client: SSHClient) -> None:
        """When conn.run() raises, run() should catch and return an error result."""
        with patch("asyncio.wait_for", side_effect=OSError("Connection refused")):
            result = await ssh_client.run("ls")
        assert result.exit_code == -1
        assert "Connection refused" in result.stderr
