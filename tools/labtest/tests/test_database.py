"""Tests for database client table-name allowlist validation.

Verifies that the VALID_TABLES allowlist in database.py correctly
prevents SQL injection through the table name parameter in get_table_count().
"""

from __future__ import annotations

import pytest

from labtest.clients.database import VALID_TABLES, DatabaseClient


class TestValidTablesAllowlist:
    """Verify the VALID_TABLES frozenset contents."""

    def test_valid_tables_is_frozenset(self) -> None:
        assert isinstance(VALID_TABLES, frozenset)

    def test_valid_tables_contains_expected_tables(self) -> None:
        expected = {
            "lab_templates",
            "pods",
            "sessions",
            "checkpoint_progress",
            "events",
            "users",
        }
        assert VALID_TABLES == expected

    def test_valid_tables_is_immutable(self) -> None:
        with pytest.raises(AttributeError):
            VALID_TABLES.add("malicious_table")  # type: ignore[attr-defined]


class TestGetTableCountValidation:
    """Test the table name validation in get_table_count().

    These tests mock the connection pool so we never touch a real database.
    The goal is to verify the ValueError is raised *before* any SQL is built.
    """

    @pytest.fixture
    def db_client(self) -> DatabaseClient:
        """Create a DatabaseClient with a dummy DSN and fake pool."""
        from unittest.mock import AsyncMock, MagicMock

        client = DatabaseClient("postgresql://test:test@localhost/test")
        # Provide a mock pool so the method can attempt pool.acquire()
        # (but we expect it to raise ValueError before reaching the pool)
        mock_pool = MagicMock()
        mock_conn = AsyncMock()
        mock_conn.fetchval = AsyncMock(return_value=42)

        # Make pool.acquire() return an async context manager
        mock_ctx = AsyncMock()
        mock_ctx.__aenter__ = AsyncMock(return_value=mock_conn)
        mock_ctx.__aexit__ = AsyncMock(return_value=False)
        mock_pool.acquire.return_value = mock_ctx

        client._pool = mock_pool
        return client

    @pytest.mark.parametrize("table", sorted(VALID_TABLES))
    async def test_valid_table_names_accepted(
        self, db_client: DatabaseClient, table: str
    ) -> None:
        """Each table in the allowlist should be accepted without error."""
        result = await db_client.get_table_count(table)
        assert result == 42

    @pytest.mark.parametrize(
        "table,reason",
        [
            ("nonexistent", "table not in allowlist"),
            ("", "empty string"),
            ("USERS", "case-sensitive mismatch"),
            ("users ", "trailing whitespace"),
            (" users", "leading whitespace"),
        ],
    )
    async def test_invalid_table_names_rejected(
        self, db_client: DatabaseClient, table: str, reason: str
    ) -> None:
        """Tables not in the allowlist must raise ValueError."""
        with pytest.raises(ValueError, match="not in the allowlist"):
            await db_client.get_table_count(table)

    @pytest.mark.parametrize(
        "injection",
        [
            "users; DROP TABLE users",
            "users; DROP TABLE users --",
            "users UNION SELECT * FROM information_schema.tables",
            "users' OR '1'='1",
            "users; DELETE FROM pods;",
            "users\n; DROP TABLE sessions",
            "' OR 1=1 --",
            "users; SELECT pg_sleep(10)",
        ],
    )
    async def test_sql_injection_attempts_blocked(
        self, db_client: DatabaseClient, injection: str
    ) -> None:
        """SQL injection attempts through the table name must raise ValueError."""
        with pytest.raises(ValueError, match="not in the allowlist"):
            await db_client.get_table_count(injection)

    async def test_error_message_includes_valid_tables(
        self, db_client: DatabaseClient
    ) -> None:
        """The ValueError message should list the valid table names."""
        with pytest.raises(ValueError) as exc_info:
            await db_client.get_table_count("fake_table")
        error_msg = str(exc_info.value)
        assert "Valid tables:" in error_msg
        # Check at least some known tables appear in the error
        assert "pods" in error_msg
        assert "users" in error_msg

    async def test_pool_not_accessed_for_invalid_table(
        self, db_client: DatabaseClient
    ) -> None:
        """The pool should never be touched when the table name is invalid."""
        with pytest.raises(ValueError):
            await db_client.get_table_count("bad_table")
        # acquire() should not have been called
        db_client._pool.acquire.assert_not_called()
