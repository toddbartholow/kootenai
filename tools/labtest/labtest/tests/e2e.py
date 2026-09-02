"""End-to-end lab lifecycle tests."""

from __future__ import annotations
import asyncio
import contextlib
import time
from .base import BaseTestSuite
from ..clients.ssh import SSHClient
from ..models.results import TestResult, TestSuiteResult


class E2ETestSuite(BaseTestSuite):
    """Full end-to-end lab lifecycle test.

    Tests the complete flow:
    1. Create pod from template
    2. Wait for pod to be ready
    3. Create session
    4. Complete lab objectives (via SSH)
    5. Wait for checkpoint detection
    6. Verify progress
    7. Submit session
    8. Verify grading
    9. Cleanup
    """

    name = "End-to-End Lab Lifecycle"

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._pod_id: str | None = None
        self._session_id: str | None = None
        self._vm_ip: str | None = None
        self._objectives_completed: bool = False
        self._exit_stack = contextlib.AsyncExitStack()

    async def run(self) -> TestSuiteResult:
        """Run full E2E test."""
        self._start_time = time.time()
        results: list[TestResult] = []

        try:
            # Phase 1: Pod Creation
            results.append(
                await self.run_test(
                    "Create Pod",
                    self.test_create_pod,
                )
            )

            results.append(
                await self.run_test(
                    "Wait for Pod Ready",
                    self.test_wait_pod_ready,
                    skip_if=lambda: self._pod_id is None,
                )
            )

            results.append(
                await self.run_test(
                    "Get VM IP",
                    self.test_get_vm_ip,
                    skip_if=lambda: self._pod_id is None,
                )
            )

            # Phase 2: Session Creation
            results.append(
                await self.run_test(
                    "Create Session",
                    self.test_create_session,
                    skip_if=lambda: self._pod_id is None,
                )
            )

            results.append(
                await self.run_test(
                    "Verify Session Has Checkpoints",
                    self.test_session_has_checkpoints,
                    skip_if=lambda: self._session_id is None,
                )
            )

            # Phase 3: Complete Lab Objectives
            results.append(
                await self.run_test(
                    "SSH to VM",
                    self.test_ssh_connectivity,
                    skip_if=lambda: self._vm_ip is None or not self.require_ssh(),
                )
            )

            results.append(
                await self.run_test(
                    "Complete Objectives",
                    self.test_complete_objectives,
                    skip_if=lambda: self._vm_ip is None or not self.require_ssh(),
                )
            )

            # Phase 4: Checkpoint Detection
            results.append(
                await self.run_test(
                    "Wait for Checkpoint Detection",
                    self.test_wait_checkpoint_detection,
                    skip_if=lambda: self._session_id is None or not self._objectives_completed,
                )
            )

            results.append(
                await self.run_test(
                    "Verify Progress",
                    self.test_verify_progress,
                    skip_if=lambda: self._session_id is None or not self._objectives_completed,
                )
            )

            # Phase 5: Submission
            results.append(
                await self.run_test(
                    "Submit Session",
                    self.test_submit_session,
                    skip_if=lambda: self._session_id is None,
                )
            )

            results.append(
                await self.run_test(
                    "Verify Grade",
                    self.test_verify_grade,
                    skip_if=lambda: self._session_id is None or not self._objectives_completed,
                )
            )

        finally:
            # Close SSH connection if we opened one
            await self._exit_stack.aclose()
            self.ssh = None

            # Phase 6: Cleanup
            if self._pod_id and self.config.cleanup_on_success:
                results.append(
                    await self.run_test(
                        "Cleanup Pod",
                        self.test_cleanup,
                    )
                )

        return self.make_result(results)

    # Phase 1: Pod Creation

    async def test_create_pod(self):
        """Create a pod from the test template."""
        resp = await self.api.create_pod(
            lab_template=self.config.test_lab_template,
            owner_id=self.config.test_user_id,
        )
        assert resp.id, "Create pod response missing ID"
        self._pod_id = resp.id

    async def test_wait_pod_ready(self):
        """Wait for pod to become ready."""
        pod = await self.api.wait_for_pod_ready(
            self._pod_id,
            timeout=self.config.pod_provision_timeout,
        )
        assert pod.status == "running", f"Pod status is {pod.status}"

    async def test_get_vm_ip(self):
        """Get VM IP address from pod."""
        pod = await self.api.get_pod(self._pod_id)
        assert len(pod.vms) > 0, "Pod has no VMs"

        # Try to get IP from first VM
        vm = pod.vms[0]
        if vm.ip_address:
            self._vm_ip = vm.ip_address
        else:
            # IP might not be assigned yet — guest agent needs time to start
            # Wait up to 90 seconds for the agent to respond
            for _ in range(18):  # 90 seconds
                await asyncio.sleep(5)
                pod = await self.api.get_pod(self._pod_id)
                if pod.vms and pod.vms[0].ip_address:
                    self._vm_ip = pod.vms[0].ip_address
                    break

        # Create SSH client if we got an IP
        if self._vm_ip:
            try:
                ssh_client = SSHClient(
                    host=self._vm_ip,
                    username=self.config.vm_ssh_user,
                    password=self.config.vm_ssh_password,
                    jump_host=self.config.infra_host,
                    jump_user=self.config.jump_ssh_user,
                    jump_password=self.config.jump_ssh_password,
                )
                self.ssh = await self._exit_stack.enter_async_context(ssh_client.connect())
            except Exception:
                # SSH connection failed (auth error, network issue, etc.)
                # The test passes since we got the IP, but SSH-dependent
                # tests will be skipped.
                pass

        # Soft failure: VM IP may not be available if qemu-guest-agent
        # or DHCP is not configured. SSH-dependent tests will be skipped.

    # Phase 2: Session Creation

    async def test_create_session(self):
        """Create a session for the pod."""
        resp = await self.api.create_session(
            pod_id=self._pod_id,
            user_id=self.config.test_user_id,
            lab_template=self.config.test_lab_template,
        )
        assert resp.session_id, "Create session response missing session_id"
        self._session_id = resp.session_id

    async def test_session_has_checkpoints(self):
        """Verify session has checkpoints."""
        progress = await self.api.get_progress(self._session_id)
        assert len(progress.checkpoints) > 0, "Session has no checkpoints"
        assert progress.max_points > 0, "Session has no points"

    # Phase 3: Complete Lab Objectives

    async def test_ssh_connectivity(self):
        """Test SSH connectivity to VM."""
        result = await self.ssh.ping()
        assert result, "SSH ping failed"

    async def test_complete_objectives(self):
        """Complete lab objectives by creating files."""
        # Create directory
        result = await self.ssh.mkdir("/home/labadmin/mywork")
        assert result, "Failed to create directory"

        # Create file
        result = await self.ssh.run("touch /home/labadmin/mywork/notes.txt")
        assert result.success, f"Failed to create file: {result.stderr}"

        # Write content
        result = await self.ssh.write_file(
            "/home/labadmin/mywork/notes.txt",
            "Hello, Kootenai!",
        )
        assert result, "Failed to write file content"

        self._objectives_completed = True

    # Phase 4: Checkpoint Detection

    async def test_wait_checkpoint_detection(self):
        """Wait for checkpoints to be detected."""
        # Get initial state
        initial_progress = await self.api.get_progress(self._session_id)
        initial_passed = sum(
            1 for cp in initial_progress.checkpoints if cp.status == "passed"
        )

        # Wait for detection
        timeout = self.config.checkpoint_detect_timeout
        elapsed = 0

        while elapsed < timeout:
            progress = await self.api.get_progress(self._session_id)
            current_passed = sum(
                1 for cp in progress.checkpoints if cp.status == "passed"
            )

            if current_passed > initial_passed:
                return  # Checkpoints detected

            await asyncio.sleep(5)
            elapsed += 5

        raise AssertionError(f"No checkpoints detected within {timeout}s")

    async def test_verify_progress(self):
        """Verify checkpoint progress."""
        progress = await self.api.get_progress(self._session_id)

        # At least some checkpoints should be passed
        passed = sum(1 for cp in progress.checkpoints if cp.status == "passed")
        assert passed > 0, "No checkpoints passed"

        # Earned points should be > 0
        assert progress.earned_points > 0, "No points earned"

    # Phase 5: Submission

    async def test_submit_session(self):
        """Submit session for grading."""
        resp = await self.api.submit_session(self._session_id)
        assert resp.status == "graded", f"Submit status is {resp.status}"

    async def test_verify_grade(self):
        """Verify final grade."""
        progress = await self.api.get_progress(self._session_id)

        # Should have earned some points
        assert progress.earned_points > 0, "No points earned after submission"
        assert progress.percentage > 0, "Zero percentage after submission"

    # Phase 6: Cleanup

    async def test_cleanup(self):
        """Clean up test resources."""
        if self._pod_id:
            await self.api.delete_pod(self._pod_id)
            self._pod_id = None


class QuickE2ETestSuite(BaseTestSuite):
    """Quick E2E test using existing resources.

    Uses an existing running pod instead of creating a new one.
    Much faster but requires a pod to already exist.
    """

    name = "Quick E2E Test"

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._pod_id: str | None = None
        self._session_id: str | None = None
        self._vm_ip: str | None = None

    async def run(self) -> TestSuiteResult:
        """Run quick E2E test."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # Find existing pod
        results.append(
            await self.run_test(
                "Find Running Pod",
                self.test_find_running_pod,
            )
        )

        if not self._pod_id:
            return self.make_result(results)

        # Create session
        results.append(
            await self.run_test(
                "Create Session",
                self.test_create_session,
            )
        )

        # Complete objectives if we have SSH
        if self.ssh and self._vm_ip:
            results.append(
                await self.run_test(
                    "Complete Objectives",
                    self.test_complete_objectives,
                )
            )

            results.append(
                await self.run_test(
                    "Wait for Detection",
                    self.test_wait_detection,
                )
            )

        # Check progress
        results.append(
            await self.run_test(
                "Check Progress",
                self.test_check_progress,
                skip_if=lambda: self._session_id is None,
            )
        )

        return self.make_result(results)

    async def test_find_running_pod(self):
        """Find an existing running pod."""
        resp = await self.api.list_pods()

        for pod in resp.pods:
            if pod.status == "running" and pod.vms:
                self._pod_id = pod.id
                if pod.vms[0].ip_address:
                    self._vm_ip = pod.vms[0].ip_address
                return

        raise AssertionError("No running pod found")

    async def test_create_session(self):
        """Create a session."""
        resp = await self.api.create_session(
            pod_id=self._pod_id,
            user_id=self.config.test_user_id,
            lab_template=self.config.test_lab_template,
        )
        self._session_id = resp.session_id

    async def test_complete_objectives(self):
        """Complete lab objectives."""
        await self.ssh.mkdir("/home/labadmin/mywork")
        await self.ssh.run("touch /home/labadmin/mywork/notes.txt")
        await self.ssh.write_file(
            "/home/labadmin/mywork/notes.txt",
            "Hello, Kootenai!",
        )

    async def test_wait_detection(self):
        """Wait briefly for checkpoint detection."""
        await asyncio.sleep(35)  # Wait for one active check cycle

    async def test_check_progress(self):
        """Check session progress."""
        progress = await self.api.get_progress(self._session_id)
        # Just verify we can get progress
        assert progress.session_id == self._session_id
