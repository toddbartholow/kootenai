"""Checkpoint detection tests."""

from __future__ import annotations
import asyncio
import time
from .base import BaseTestSuite
from ..models.results import TestResult, TestSuiteResult


class CheckpointTestSuite(BaseTestSuite):
    """Checkpoint detection tests.

    Tests that checkpoint triggers are detected when lab objectives are completed.
    Requires SSH access to the lab VM.
    """

    name = "Checkpoint Detection"

    def __init__(
        self,
        *args,
        pod_id: str | None = None,
        session_id: str | None = None,
        vm_ip: str | None = None,
        **kwargs,
    ):
        super().__init__(*args, **kwargs)
        self._pod_id = pod_id
        self._session_id = session_id
        self._vm_ip = vm_ip
        self._initial_passed_count = 0

    async def run(self) -> TestSuiteResult:
        """Run checkpoint tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # Find resources if not provided
        if not self._pod_id or not self._vm_ip:
            results.append(
                await self.run_test(
                    "Find Running Pod with IP",
                    self.test_find_pod_with_ip,
                )
            )

        if not self._pod_id:
            return self.make_result(results)

        # Create session if needed
        if not self._session_id:
            results.append(
                await self.run_test(
                    "Create Session",
                    self.test_create_session,
                )
            )

        if not self._session_id:
            return self.make_result(results)

        # Get initial checkpoint state
        results.append(
            await self.run_test(
                "Get Initial Checkpoint State",
                self.test_get_initial_state,
            )
        )

        # Test SSH connectivity
        results.append(
            await self.run_test(
                "SSH Connectivity",
                self.test_ssh_connectivity,
                skip_if=lambda: not self.require_ssh(),
            )
        )

        # Create lab files (triggers file_exists and file_content checkpoints)
        results.append(
            await self.run_test(
                "Create Lab Directory",
                self.test_create_directory,
                skip_if=lambda: not self.require_ssh(),
            )
        )

        results.append(
            await self.run_test(
                "Create Lab File",
                self.test_create_file,
                skip_if=lambda: not self.require_ssh(),
            )
        )

        results.append(
            await self.run_test(
                "Write File Content",
                self.test_write_content,
                skip_if=lambda: not self.require_ssh(),
            )
        )

        # Wait for checkpoints to be detected
        results.append(
            await self.run_test(
                "Wait for Checkpoint Detection",
                self.test_wait_for_detection,
            )
        )

        # Verify checkpoints passed
        results.append(
            await self.run_test(
                "Verify Checkpoints Passed",
                self.test_verify_checkpoints_passed,
            )
        )

        return self.make_result(results)

    async def test_find_pod_with_ip(self):
        """Find a running pod with VM IP address."""
        resp = await self.api.list_pods()

        for pod in resp.pods:
            if pod.status != "running":
                continue
            for vm in pod.vms:
                if vm.ip_address:
                    self._pod_id = pod.id
                    self._vm_ip = vm.ip_address
                    return

        raise AssertionError("No running pod with VM IP found")

    async def test_create_session(self):
        """Create a session for the pod."""
        resp = await self.api.create_session(
            pod_id=self._pod_id,
            user_id=self.config.test_user_id,
            lab_template=self.config.test_lab_template,
        )
        self._session_id = resp.session_id

    async def test_get_initial_state(self):
        """Get initial checkpoint state."""
        progress = await self.api.get_progress(self._session_id)
        self._initial_passed_count = sum(
            1 for cp in progress.checkpoints if cp.status == "passed"
        )

    async def test_ssh_connectivity(self):
        """Test SSH connectivity to VM."""
        result = await self.ssh.ping()
        assert result, "SSH ping failed"

    async def test_create_directory(self):
        """Create the lab work directory."""
        # For Simple Linux Introduction lab
        result = await self.ssh.mkdir("/home/student/mywork")
        assert result, "Failed to create directory"

    async def test_create_file(self):
        """Create the lab file."""
        result = await self.ssh.run("touch /home/student/mywork/notes.txt")
        assert result.success, f"Failed to create file: {result.stderr}"

    async def test_write_content(self):
        """Write expected content to the file."""
        result = await self.ssh.write_file(
            "/home/student/mywork/notes.txt",
            "Hello, Kootenai!",
        )
        assert result, "Failed to write file content"

    async def test_wait_for_detection(self):
        """Wait for the active check runner to detect changes."""
        # The active check runner polls every 30 seconds
        # Wait up to checkpoint_detect_timeout
        timeout = self.config.checkpoint_detect_timeout
        poll_interval = 5
        elapsed = 0

        while elapsed < timeout:
            progress = await self.api.get_progress(self._session_id)
            current_passed = sum(
                1 for cp in progress.checkpoints if cp.status == "passed"
            )

            if current_passed > self._initial_passed_count:
                return  # New checkpoints detected

            await asyncio.sleep(poll_interval)
            elapsed += poll_interval

        raise AssertionError(
            f"No new checkpoints detected within {timeout}s. "
            f"Initial: {self._initial_passed_count} checkpoints passed."
        )

    async def test_verify_checkpoints_passed(self):
        """Verify expected checkpoints are now passed."""
        progress = await self.api.get_progress(self._session_id)

        # Check specific checkpoints for Simple Linux Introduction
        expected_passed = ["create-directory", "create-file", "write-content"]
        passed_ids = [
            cp.id
            for cp in progress.checkpoints
            if cp.status == "passed"
        ]

        for expected in expected_passed:
            if expected not in passed_ids:
                raise AssertionError(
                    f"Checkpoint '{expected}' not passed. Passed checkpoints: {passed_ids}"
                )


class CheckpointDatabaseTestSuite(BaseTestSuite):
    """Verify checkpoint progress is persisted to database."""

    name = "Checkpoint Persistence"

    def __init__(self, *args, session_id: str | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        self._session_id = session_id

    async def run(self) -> TestSuiteResult:
        """Run checkpoint database tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        if not self._session_id:
            results.append(
                await self.run_test(
                    "Find Session with Progress",
                    self.test_find_session,
                )
            )

        if not self._session_id:
            return self.make_result(results)

        results.append(
            await self.run_test(
                "Progress in Database",
                self.test_progress_in_db,
            )
        )

        results.append(
            await self.run_test(
                "Passed Checkpoints Count",
                self.test_passed_count,
            )
        )

        return self.make_result(results)

    async def test_find_session(self):
        """Find a session with checkpoint progress."""
        sessions = await self.db.list_sessions()

        for session in sessions:
            progress = await self.db.get_checkpoint_progress(session["id"])
            if progress:
                self._session_id = session["id"]
                return

        # If no session with progress, use the most recent
        if sessions:
            self._session_id = sessions[0]["id"]
        else:
            raise AssertionError("No sessions found in database")

    async def test_progress_in_db(self):
        """Verify checkpoint progress is in database."""
        progress = await self.db.get_checkpoint_progress(self._session_id)
        assert isinstance(progress, list), "Progress should be a list"

    async def test_passed_count(self):
        """Verify passed checkpoint count matches API."""
        db_progress = await self.db.get_checkpoint_progress(self._session_id)
        db_passed = sum(1 for p in db_progress if p.get("status") == "passed")

        api_progress = await self.api.get_progress(self._session_id)
        api_passed = sum(
            1 for cp in api_progress.checkpoints if cp.status == "passed"
        )

        assert db_passed == api_passed, (
            f"Database has {db_passed} passed checkpoints, "
            f"API shows {api_passed}"
        )
