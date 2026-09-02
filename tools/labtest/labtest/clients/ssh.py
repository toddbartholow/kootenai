"""SSH client for VM access and file operations."""

from __future__ import annotations
import asyncio
import logging
import shlex
from contextlib import asynccontextmanager
from dataclasses import dataclass

import asyncssh

logger = logging.getLogger(__name__)


@dataclass
class CommandResult:
    """Result of an SSH command execution."""

    exit_code: int
    stdout: str
    stderr: str

    @property
    def success(self) -> bool:
        return self.exit_code == 0


class SSHClient:
    """Async SSH client for VM access.

    Supports connecting directly or via a jump host (infra VM).

    Usage:
        # Direct connection
        async with SSHClient(host, user, password).connect() as ssh:
            result = await ssh.run("ls -la")

        # Via jump host
        async with SSHClient(vm_ip, user, password, jump_host=infra_ip).connect() as ssh:
            result = await ssh.run("cat /etc/hostname")
    """

    def __init__(
        self,
        host: str,
        username: str,
        password: str,
        port: int = 22,
        jump_host: str | None = None,
        jump_user: str | None = None,
        jump_password: str | None = None,
    ):
        """Initialize SSH client.

        Args:
            host: Target host IP or hostname
            username: SSH username
            password: SSH password
            port: SSH port (default 22)
            jump_host: Optional jump host for SSH tunneling
            jump_user: Jump host username (defaults to same as target)
            jump_password: Jump host password (defaults to same as target)
        """
        self.host = host
        self.username = username
        self.password = password
        self.port = port
        self.jump_host = jump_host
        self.jump_user = jump_user or username
        self.jump_password = jump_password or password
        self._conn: asyncssh.SSHClientConnection | None = None
        self._jump_conn: asyncssh.SSHClientConnection | None = None

    @asynccontextmanager
    async def connect(self):
        """Context manager for SSH connection."""
        try:
            if self.jump_host:
                # Connect via jump host
                self._jump_conn = await asyncssh.connect(
                    self.jump_host,
                    username=self.jump_user,
                    password=self.jump_password,
                    known_hosts=None,
                )
                self._conn = await self._jump_conn.connect_ssh(
                    self.host,
                    username=self.username,
                    password=self.password,
                    known_hosts=None,
                )
            else:
                # Direct connection
                self._conn = await asyncssh.connect(
                    self.host,
                    port=self.port,
                    username=self.username,
                    password=self.password,
                    known_hosts=None,
                )
            yield self
        finally:
            if self._conn:
                self._conn.close()
                self._conn = None
            if self._jump_conn:
                self._jump_conn.close()
                self._jump_conn = None

    async def run(self, command: str, timeout: int = 30) -> CommandResult:
        """Execute a command on the remote host.

        Args:
            command: Shell command to execute
            timeout: Command timeout in seconds

        Returns:
            CommandResult with exit code, stdout, stderr
        """
        try:
            result = await asyncio.wait_for(
                self._conn.run(command),
                timeout=timeout,
            )
            return CommandResult(
                exit_code=result.exit_status or 0,
                stdout=result.stdout or "",
                stderr=result.stderr or "",
            )
        except asyncio.TimeoutError:
            return CommandResult(
                exit_code=-1,
                stdout="",
                stderr=f"Command timed out after {timeout}s",
            )
        except Exception as e:
            return CommandResult(
                exit_code=-1,
                stdout="",
                stderr=str(e),
            )

    async def file_exists(self, path: str) -> bool:
        """Check if a file or directory exists.

        Args:
            path: Path to check

        Returns:
            True if path exists
        """
        result = await self.run(f"test -e {shlex.quote(path)} && echo 'exists'")
        return "exists" in result.stdout

    async def read_file(self, path: str) -> str | None:
        """Read file contents.

        Args:
            path: Path to file

        Returns:
            File contents or None if file doesn't exist
        """
        result = await self.run(f"cat {shlex.quote(path)}")
        if result.success:
            return result.stdout
        return None

    async def write_file(self, path: str, content: str) -> bool:
        """Write content to a file.

        Args:
            path: Path to file
            content: Content to write

        Returns:
            True if successful
        """
        result = await self.run(f"echo {shlex.quote(content)} > {shlex.quote(path)}")
        return result.success

    async def mkdir(self, path: str, parents: bool = True) -> bool:
        """Create a directory.

        Args:
            path: Path to directory
            parents: Create parent directories if needed

        Returns:
            True if successful
        """
        flags = "-p" if parents else ""
        result = await self.run(f"mkdir {flags} {shlex.quote(path)}")
        return result.success

    async def remove(self, path: str, recursive: bool = False) -> bool:
        """Remove a file or directory.

        Args:
            path: Path to remove
            recursive: Remove directories recursively

        Returns:
            True if successful
        """
        flags = "-rf" if recursive else "-f"
        result = await self.run(f"rm {flags} {shlex.quote(path)}")
        return result.success

    async def file_contains(self, path: str, pattern: str) -> bool:
        """Check if file contains a pattern.

        Args:
            path: Path to file
            pattern: Pattern to search for

        Returns:
            True if pattern found in file
        """
        result = await self.run(f"grep -q {shlex.quote(pattern)} {shlex.quote(path)}")
        return result.success

    async def get_hostname(self) -> str:
        """Get hostname of the remote machine.

        Returns:
            Hostname string
        """
        result = await self.run("hostname")
        return result.stdout.strip()

    async def ping(self) -> bool:
        """Test SSH connectivity.

        Returns:
            True if connection is working
        """
        result = await self.run("echo 'pong'")
        return result.success and "pong" in result.stdout
