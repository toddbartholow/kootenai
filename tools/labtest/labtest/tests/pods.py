"""Pod lifecycle tests."""

from __future__ import annotations
import time
from typing import Optional

from .base import BaseTestSuite
from ..models.results import TestResult, TestSuiteResult


class PodTestSuite(BaseTestSuite):
    """Pod lifecycle tests.

    Tests pod creation, status checking, and deletion.
    These are slower tests that create real VMs.
    """

    name = "Pod Lifecycle"

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._created_pod_id: Optional[str] = None

    async def run(self) -> TestSuiteResult:
        """Run all pod tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        try:
            # List existing pods
            results.append(
                await self.run_test(
                    "List Pods",
                    self.test_list_pods,
                    skip_if=lambda: not self.require_api(),
                )
            )

            # Create pod
            results.append(
                await self.run_test(
                    "Create Pod",
                    self.test_create_pod,
                    skip_if=lambda: not self.require_api(),
                )
            )

            # Wait for pod ready
            results.append(
                await self.run_test(
                    "Wait Pod Ready",
                    self.test_wait_pod_ready,
                    skip_if=lambda: self._created_pod_id is None,
                    skip_reason="Pod creation failed",
                )
            )

            # Get pod details
            results.append(
                await self.run_test(
                    "Get Pod",
                    self.test_get_pod,
                    skip_if=lambda: self._created_pod_id is None,
                    skip_reason="Pod creation failed",
                )
            )

            # Verify pod has VMs
            results.append(
                await self.run_test(
                    "Pod Has VMs",
                    self.test_pod_has_vms,
                    skip_if=lambda: self._created_pod_id is None,
                    skip_reason="Pod creation failed",
                )
            )

        finally:
            # Cleanup
            if self._created_pod_id and self.config.cleanup_on_success:
                results.append(
                    await self.run_test(
                        "Delete Pod",
                        self.test_delete_pod,
                    )
                )

        return self.make_result(results)

    async def test_list_pods(self):
        """Test listing pods."""
        resp = await self.api.list_pods()
        # Just verify we get a response with pods list
        assert hasattr(resp, "pods"), "Response missing pods field"

    async def test_create_pod(self):
        """Test creating a pod from template."""
        resp = await self.api.create_pod(
            lab_template=self.config.test_lab_template,
            owner_id=self.config.test_user_id,
        )
        assert resp.id, "Create pod response missing ID"
        self._created_pod_id = resp.id

    async def test_wait_pod_ready(self):
        """Test waiting for pod to become ready."""
        pod = await self.api.wait_for_pod_ready(
            self._created_pod_id,
            timeout=self.config.pod_provision_timeout,
        )
        assert pod.status == "running", f"Pod status is {pod.status}, expected running"

    async def test_get_pod(self):
        """Test getting pod details."""
        pod = await self.api.get_pod(self._created_pod_id)
        assert pod.id == self._created_pod_id, "Pod ID mismatch"
        assert pod.status in ("running", "provisioning"), f"Unexpected status: {pod.status}"

    async def test_pod_has_vms(self):
        """Test that pod has VMs."""
        pod = await self.api.get_pod(self._created_pod_id)
        assert len(pod.vms) > 0, "Pod has no VMs"

        # Check VM has required fields
        vm = pod.vms[0]
        assert vm.name, "VM missing name"
        assert vm.platform_id, "VM missing platform_id"

    async def test_delete_pod(self):
        """Test deleting a pod."""
        if self._created_pod_id:
            await self.api.delete_pod(self._created_pod_id)
            self._created_pod_id = None


class PodOperationsTestSuite(BaseTestSuite):
    """Advanced pod operations tests.

    Tests start/stop/reset operations on an existing pod.
    """

    name = "Pod Operations"

    def __init__(self, *args, pod_id: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        self._pod_id = pod_id

    async def run(self) -> TestSuiteResult:
        """Run pod operations tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # Find or use provided pod
        if not self._pod_id:
            results.append(
                await self.run_test(
                    "Find Running Pod",
                    self.test_find_running_pod,
                )
            )

        if self._pod_id:
            results.append(
                await self.run_test(
                    "Stop Pod",
                    self.test_stop_pod,
                )
            )

            results.append(
                await self.run_test(
                    "Start Pod",
                    self.test_start_pod,
                )
            )

            results.append(
                await self.run_test(
                    "Reset Pod",
                    self.test_reset_pod,
                )
            )

        return self.make_result(results)

    async def test_find_running_pod(self):
        """Find a running pod to test operations on."""
        resp = await self.api.list_pods()
        running_pods = [p for p in resp.pods if p.status == "running"]

        if not running_pods:
            raise AssertionError("No running pods found for operations testing")

        self._pod_id = running_pods[0].id

    async def test_stop_pod(self):
        """Test stopping a pod."""
        await self.api.stop_pod(self._pod_id)
        # Verify pod is stopped or stopping
        pod = await self.api.get_pod(self._pod_id)
        assert pod.status in ("stopped", "stopping"), f"Pod status is {pod.status}"

    async def test_start_pod(self):
        """Test starting a pod."""
        await self.api.start_pod(self._pod_id)
        # Wait briefly and verify
        pod = await self.api.wait_for_pod_ready(self._pod_id, timeout=60)
        assert pod.status == "running", f"Pod status is {pod.status}"

    async def test_reset_pod(self):
        """Test resetting a pod to initial state."""
        await self.api.reset_pod(self._pod_id)
        # Should still be running after reset
        pod = await self.api.get_pod(self._pod_id)
        assert pod.status in ("running", "provisioning"), f"Pod status is {pod.status}"
