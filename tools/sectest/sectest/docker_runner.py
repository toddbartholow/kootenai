"""Docker container runner for security scanners."""

from __future__ import annotations

import json
import subprocess
import time
from pathlib import Path
from typing import Optional

from rich.console import Console

from .config import SectestConfig

console = Console()


class DockerRunner:
    """Runs scanner containers via Docker CLI."""

    def __init__(self, config: SectestConfig):
        self.config = config
        self.docker = config.docker_bin

    def is_available(self) -> bool:
        """Check if Docker is available."""
        try:
            result = subprocess.run(
                [self.docker, "version", "--format", "{{.Server.Version}}"],
                capture_output=True,
                text=True,
                timeout=10,
            )
            return result.returncode == 0
        except (subprocess.TimeoutExpired, FileNotFoundError):
            return False

    def pull_image(self, image: str) -> bool:
        """Pull a Docker image if not present."""
        # Check if image exists locally
        check = subprocess.run(
            [self.docker, "image", "inspect", image],
            capture_output=True,
            text=True,
        )
        if check.returncode == 0:
            return True

        console.print(f"  Pulling [cyan]{image}[/]...")
        result = subprocess.run(
            [self.docker, "pull", image],
            capture_output=True,
            text=True,
            timeout=300,
        )
        return result.returncode == 0

    def run(
        self,
        image: str,
        command: list[str],
        *,
        volumes: Optional[dict[str, str]] = None,
        env: Optional[dict[str, str]] = None,
        network: Optional[str] = None,
        timeout: Optional[int] = None,
        name: Optional[str] = None,
    ) -> tuple[int, str, str]:
        """Run a container and return (exit_code, stdout, stderr)."""
        cmd = [self.docker, "run", "--rm"]

        if name:
            cmd.extend(["--name", name])

        net = network or self.config.docker_network
        cmd.extend(["--network", net])

        if volumes:
            for host_path, container_path in volumes.items():
                cmd.extend(["-v", f"{host_path}:{container_path}"])

        if env:
            for key, val in env.items():
                cmd.extend(["-e", f"{key}={val}"])

        cmd.append(image)
        cmd.extend(command)

        if self.config.verbose:
            console.print(f"  [dim]$ {' '.join(cmd)}[/]")

        t = timeout or self.config.scan_timeout
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=t,
            )
            return result.returncode, result.stdout, result.stderr
        except subprocess.TimeoutExpired:
            return -1, "", f"Container timed out after {t}s"

    def ensure_images(self, images: list[str]) -> dict[str, bool]:
        """Pull multiple images, return availability map."""
        results = {}
        for img in images:
            results[img] = self.pull_image(img)
        return results
