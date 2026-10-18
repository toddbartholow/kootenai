"""Session management tests."""

from __future__ import annotations
import time
from typing import Optional

from .base import BaseTestSuite
from ..models.results import TestResult, TestSuiteResult


class SessionTestSuite(BaseTestSuite):
    """Session management tests.

    Tests session creation, progress tracking, and submission.
    Requires an existing pod.
    """

    name = "Session Management"

    def __init__(self, *args, pod_id: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        self._pod_id = pod_id
        self._session_id: Optional[str] = None

    async def run(self) -> TestSuiteResult:
        """Run all session tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # Find a pod if not provided
        if not self._pod_id:
            results.append(
                await self.run_test(
                    "Find Running Pod",
                    self.test_find_running_pod,
                )
            )

        if not self._pod_id:
            # No pod available, skip remaining tests
            return self.make_result(results)

        # List sessions
        results.append(
            await self.run_test(
                "List Sessions",
                self.test_list_sessions,
            )
        )

        # Create session
        results.append(
            await self.run_test(
                "Create Session",
                self.test_create_session,
            )
        )

        # Get session
        results.append(
            await self.run_test(
                "Get Session",
                self.test_get_session,
                skip_if=lambda: self._session_id is None,
                skip_reason="Session creation failed",
            )
        )

        # Get progress
        results.append(
            await self.run_test(
                "Get Progress",
                self.test_get_progress,
                skip_if=lambda: self._session_id is None,
                skip_reason="Session creation failed",
            )
        )

        # Verify checkpoints exist
        results.append(
            await self.run_test(
                "Has Checkpoints",
                self.test_has_checkpoints,
                skip_if=lambda: self._session_id is None,
                skip_reason="Session creation failed",
            )
        )

        # Submit session
        results.append(
            await self.run_test(
                "Submit Session",
                self.test_submit_session,
                skip_if=lambda: self._session_id is None,
                skip_reason="Session creation failed",
            )
        )

        return self.make_result(results)

    async def test_find_running_pod(self):
        """Find a running pod for session testing."""
        resp = await self.api.list_pods()
        running_pods = [p for p in resp.pods if p.status == "running"]

        if not running_pods:
            raise AssertionError("No running pods found for session testing")

        self._pod_id = running_pods[0].id

    async def test_list_sessions(self):
        """Test listing sessions."""
        resp = await self.api.list_sessions()
        # Just verify we get a response
        assert hasattr(resp, "sessions"), "Response missing sessions field"

    async def test_create_session(self):
        """Test creating a session."""
        resp = await self.api.create_session(
            pod_id=self._pod_id,
            user_id=self.config.test_user_id,
            lab_template=self.config.test_lab_template,
        )
        assert resp.session_id, "Create session response missing session_id"
        self._session_id = resp.session_id

    async def test_get_session(self):
        """Test getting session details."""
        session = await self.api.get_session(self._session_id)
        assert session.id == self._session_id, "Session ID mismatch"
        assert session.pod_id == self._pod_id, "Pod ID mismatch"

    async def test_get_progress(self):
        """Test getting session progress."""
        progress = await self.api.get_progress(self._session_id)
        assert progress.session_id == self._session_id, "Session ID mismatch"
        assert progress.max_points >= 0, "Invalid max_points"
        assert progress.earned_points >= 0, "Invalid earned_points"

    async def test_has_checkpoints(self):
        """Test that session has checkpoints."""
        progress = await self.api.get_progress(self._session_id)
        assert len(progress.checkpoints) > 0, "Session has no checkpoints"

    async def test_submit_session(self):
        """Test submitting session for grading."""
        resp = await self.api.submit_session(self._session_id)
        assert resp.status == "graded", f"Submit status is {resp.status}"
        assert resp.max_points >= 0, "Invalid max_points"


class SessionDatabaseTestSuite(BaseTestSuite):
    """Session database verification tests.

    Verifies session data is correctly persisted to database.
    """

    name = "Session Database"

    def __init__(self, *args, session_id: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        self._session_id = session_id

    async def run(self) -> TestSuiteResult:
        """Run session database tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # Find a session if not provided
        if not self._session_id:
            results.append(
                await self.run_test(
                    "Find Session",
                    self.test_find_session,
                )
            )

        if not self._session_id:
            return self.make_result(results)

        # Verify session in database
        results.append(
            await self.run_test(
                "Session in Database",
                self.test_session_in_db,
            )
        )

        # Verify checkpoint progress
        results.append(
            await self.run_test(
                "Checkpoint Progress in Database",
                self.test_checkpoint_progress_in_db,
            )
        )

        return self.make_result(results)

    async def test_find_session(self):
        """Find a session in database."""
        sessions = await self.db.list_sessions()
        if not sessions:
            raise AssertionError("No sessions found in database")
        self._session_id = sessions[0]["id"]

    async def test_session_in_db(self):
        """Verify session exists in database."""
        session = await self.db.get_session(self._session_id)
        assert session is not None, "Session not found in database"
        assert session["id"] == self._session_id, "Session ID mismatch"

    async def test_checkpoint_progress_in_db(self):
        """Verify checkpoint progress in database."""
        progress = await self.db.get_checkpoint_progress(self._session_id)
        # Progress may be empty if no checkpoints have been completed
        assert isinstance(progress, list), "Invalid progress data"
