"""Async PostgreSQL client for Kootenai database."""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import Any

import asyncpg

logger = logging.getLogger(__name__)

# Allowlist of tables that may be queried via get_table_count().
# This prevents SQL injection through the table name parameter.
VALID_TABLES: frozenset[str] = frozenset({
    "lab_templates",
    "pods",
    "sessions",
    "checkpoint_progress",
    "events",
    "users",
})


class DatabaseClient:
    """Async PostgreSQL client for direct database access.

    Usage:
        async with DatabaseClient(dsn).session() as db:
            await db.ping()
    """

    def __init__(self, dsn: str):
        """Initialize database client.

        Args:
            dsn: PostgreSQL connection string
                 e.g., postgresql://user:pass@host:5432/dbname
        """
        self.dsn = dsn
        self._pool: asyncpg.Pool | None = None

    @asynccontextmanager
    async def session(self):
        """Context manager for database connection pool."""
        self._pool = await asyncpg.create_pool(
            self.dsn,
            min_size=1,
            max_size=5,
            command_timeout=30,
        )
        try:
            yield self
        finally:
            await self._pool.close()
            self._pool = None

    async def ping(self) -> bool:
        """Test database connectivity.

        Returns:
            True if connection is successful
        """
        async with self._pool.acquire() as conn:
            result = await conn.fetchval("SELECT 1")
            return result == 1

    async def get_table_count(self, table: str) -> int:
        """Get row count for a table.

        Args:
            table: Table name (must be in VALID_TABLES allowlist)

        Returns:
            Number of rows in table

        Raises:
            ValueError: If table name is not in the allowlist
        """
        if table not in VALID_TABLES:
            raise ValueError(
                f"Table {table!r} is not in the allowlist. "
                f"Valid tables: {sorted(VALID_TABLES)}"
            )
        async with self._pool.acquire() as conn:
            # Table name is validated against the allowlist above,
            # so identifier interpolation is safe here.
            result = await conn.fetchval(f"SELECT COUNT(*) FROM {table}")
            return result or 0

    # Lab Templates

    async def list_lab_templates(self, active_only: bool = True) -> list[dict[str, Any]]:
        """List lab templates from database.

        Args:
            active_only: Only return active templates

        Returns:
            List of lab template records
        """
        async with self._pool.acquire() as conn:
            query = "SELECT * FROM lab_templates"
            if active_only:
                query += " WHERE is_active = true"
            query += " ORDER BY name"
            rows = await conn.fetch(query)
            return [dict(row) for row in rows]

    async def get_lab_template(self, template_id: str) -> dict[str, Any] | None:
        """Get lab template by ID.

        Args:
            template_id: UUID of the template

        Returns:
            Template record or None
        """
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT * FROM lab_templates WHERE id = $1",
                template_id,
            )
            return dict(row) if row else None

    # Pods

    async def list_pods(self, status: str | None = None) -> list[dict[str, Any]]:
        """List pods from database.

        Args:
            status: Filter by status (optional)

        Returns:
            List of pod records
        """
        async with self._pool.acquire() as conn:
            query = "SELECT * FROM pods"
            if status:
                query += " WHERE status = $1"
                rows = await conn.fetch(query, status)
            else:
                rows = await conn.fetch(query)
            return [dict(row) for row in rows]

    async def get_pod(self, pod_id: str) -> dict[str, Any] | None:
        """Get pod by ID.

        Args:
            pod_id: UUID of the pod

        Returns:
            Pod record or None
        """
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow("SELECT * FROM pods WHERE id = $1", pod_id)
            return dict(row) if row else None

    # Sessions

    async def list_sessions(
        self,
        user_id: str | None = None,
        pod_id: str | None = None,
    ) -> list[dict[str, Any]]:
        """List sessions from database.

        Args:
            user_id: Filter by user ID (optional)
            pod_id: Filter by pod ID (optional)

        Returns:
            List of session records
        """
        async with self._pool.acquire() as conn:
            conditions = []
            params = []
            param_num = 1

            if user_id:
                conditions.append(f"user_id = ${param_num}")
                params.append(user_id)
                param_num += 1

            if pod_id:
                conditions.append(f"pod_id = ${param_num}")
                params.append(pod_id)
                param_num += 1

            query = "SELECT * FROM sessions"
            if conditions:
                query += " WHERE " + " AND ".join(conditions)
            query += " ORDER BY started_at DESC"

            rows = await conn.fetch(query, *params)
            return [dict(row) for row in rows]

    async def get_session(self, session_id: str) -> dict[str, Any] | None:
        """Get session by ID.

        Args:
            session_id: UUID of the session

        Returns:
            Session record or None
        """
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow("SELECT * FROM sessions WHERE id = $1", session_id)
            return dict(row) if row else None

    # Checkpoint Progress

    async def get_checkpoint_progress(self, session_id: str) -> list[dict[str, Any]]:
        """Get checkpoint progress for a session.

        Args:
            session_id: UUID of the session

        Returns:
            List of checkpoint progress records
        """
        async with self._pool.acquire() as conn:
            rows = await conn.fetch(
                """
                SELECT * FROM checkpoint_progress
                WHERE session_id = $1
                ORDER BY checkpoint_id
                """,
                session_id,
            )
            return [dict(row) for row in rows]

    async def count_passed_checkpoints(self, session_id: str) -> int:
        """Count passed checkpoints for a session.

        Args:
            session_id: UUID of the session

        Returns:
            Number of passed checkpoints
        """
        async with self._pool.acquire() as conn:
            result = await conn.fetchval(
                """
                SELECT COUNT(*) FROM checkpoint_progress
                WHERE session_id = $1 AND status = 'passed'
                """,
                session_id,
            )
            return result or 0

    # Events

    async def get_recent_events(self, limit: int = 100) -> list[dict[str, Any]]:
        """Get recent events from the event store.

        Args:
            limit: Maximum number of events to return

        Returns:
            List of event records
        """
        async with self._pool.acquire() as conn:
            rows = await conn.fetch(
                """
                SELECT * FROM events
                ORDER BY created_at DESC
                LIMIT $1
                """,
                limit,
            )
            return [dict(row) for row in rows]

    # Health check query

    async def check_tables_exist(self) -> dict[str, bool]:
        """Check if required tables exist.

        Returns:
            Dict mapping table name to existence status
        """
        # Core tables expected in the Kootenai database
        # Some tables may have different names depending on migrations
        tables = [
            "lab_templates",
            "pods",
            "checkpoint_progress",
            "events",
            "users",
        ]

        result = {}
        async with self._pool.acquire() as conn:
            for table in tables:
                exists = await conn.fetchval(
                    """
                    SELECT EXISTS (
                        SELECT FROM information_schema.tables
                        WHERE table_schema = 'public'
                        AND table_name = $1
                    )
                    """,
                    table,
                )
                result[table] = exists

        return result
