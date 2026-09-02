"""Base scanner interface."""

from __future__ import annotations

import time
from abc import ABC, abstractmethod

from ..config import SectestConfig
from ..docker_runner import DockerRunner
from ..models import ScanResult, ScanStatus


class BaseScanner(ABC):
    """Abstract base for all scanners."""

    name: str = "base"
    docker_image: str = ""

    def __init__(self, config: SectestConfig, runner: DockerRunner):
        self.config = config
        self.runner = runner

    @abstractmethod
    def scan(self) -> ScanResult:
        """Run the scan and return results."""
        ...

    def timed_scan(self) -> ScanResult:
        """Wrapper that adds timing to scan()."""
        start = time.time()
        try:
            result = self.scan()
        except Exception as e:
            result = ScanResult(
                scanner=self.name,
                status=ScanStatus.ERROR,
                error=str(e),
            )
        result.duration = time.time() - start
        return result

    def is_available(self) -> bool:
        """Check if the scanner's Docker image is available."""
        if not self.docker_image:
            return True  # Native scanner, no Docker needed
        return self.runner.pull_image(self.docker_image)
