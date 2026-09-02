"""Custom exception hierarchy for labtest.

Provides structured error types so callers can catch specific failure
categories rather than relying on bare Exception or RuntimeError.
"""

from __future__ import annotations


class LabtestError(Exception):
    """Base exception for all labtest errors."""


class ConfigError(LabtestError):
    """Raised when configuration is invalid or missing."""


class LabConnectionError(LabtestError):
    """Raised when a connection to a service (API, DB, NATS, SSH) fails.

    Named ``LabConnectionError`` to avoid shadowing the builtin
    ``ConnectionError``.
    """


class PodError(LabtestError):
    """Raised when a pod operation fails (e.g. enters error state)."""


class TimeoutError(LabtestError):  # noqa: A001 — intentional shadow of builtin for domain clarity
    """Raised when an operation exceeds its deadline."""
