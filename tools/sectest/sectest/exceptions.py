"""Custom exception hierarchy for sectest.

Provides a structured base exception so callers and reporters can
distinguish sectest failures from unexpected errors.
"""

from __future__ import annotations


class SectestError(Exception):
    """Base exception for all sectest errors."""


class ScanError(SectestError):
    """Raised when a scanner encounters a fatal error."""


class DockerError(SectestError):
    """Raised when a Docker operation fails."""
