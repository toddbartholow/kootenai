"""Simulator test suite: orchestrates execution and verification."""

from __future__ import annotations

import asyncio
import contextlib
import logging
import time
from pathlib import Path
from ..clients.api import APIClient
from ..clients.ssh import SSHClient
from ..config import LabtestConfig
from ..models.results import TestResult, TestStatus, TestSuiteResult
from ..tests.base import BaseTestSuite
from .actions import generate_actions
from .parser import Objective, ParsedTemplate, VMSpec, parse_template_file
from .scenario import Scenario, load_scenario

logger = logging.getLogger(__name__)


class SimulatorTestSuite(BaseTestSuite):
    """Simulate student actions and verify checkpoint triggers.

    Parses a lab template, auto-generates SSH commands from trigger definitions,
    executes them on the correct VMs, and verifies checkpoint completion via the API.
    """

    name = "Lab Simulator"

    def __init__(
        self,
        config: LabtestConfig,
        *,
        api: APIClient | None = None,
        template_path: str | None = None,
        template_data: dict | None = None,
        scenario_path: str | None = None,
        pod_id: str | None = None,
        dry_run: bool = False,
        no_cleanup: bool = False,
        ssh_user: str | None = None,
        ssh_password: str | None = None,
        timeout: int = 120,
    ):
        super().__init__(config, api=api)
        self._template_path = template_path
        self._template_data = template_data
        self._scenario_path = scenario_path
        self._pod_id = pod_id
        self._dry_run = dry_run
        self._no_cleanup = no_cleanup
        self._ssh_user = ssh_user or config.vm_ssh_user
        self._ssh_password = ssh_password or config.vm_ssh_password
        self._timeout = timeout

        self._template: ParsedTemplate | None = None
        self._scenario: Scenario | None = None
        self._session_id: str | None = None
        self._vm_ssh_clients: dict[str, SSHClient] = {}
        self._exit_stack = contextlib.AsyncExitStack()
        self._vm_ip_map: dict[str, str] = {}
        self._created_pod: bool = False
        # Collect generated plan for dry-run output
        self._action_plan: list[dict] = []

    async def run(self) -> TestSuiteResult:
        """Run the simulation."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # 1. Load template
        results.append(await self.run_test("Load Template", self._load_template))
        if self._template is None:
            return self.make_result(results)

        # 2. Load scenario (optional)
        if self._scenario_path:
            results.append(await self.run_test("Load Scenario", self._load_scenario))

        # 3. Generate action plan
        results.append(await self.run_test("Generate Action Plan", self._generate_plan))

        if self._dry_run:
            # Dry run: just print the plan and return
            results.append(await self.run_test("Display Action Plan", self._display_plan))
            return self.make_result(results)

        # 4. Provision pod (or use existing)
        results.append(await self.run_test("Provision Pod", self._provision_pod))
        if not self._pod_id:
            return self.make_result(results)

        # 5. Create session
        results.append(await self.run_test("Create Session", self._create_session))
        if not self._session_id:
            return self.make_result(results)

        # 6. Connect SSH to VMs
        results.append(await self.run_test("Connect SSH", self._connect_ssh))

        # 7. Execute actions
        results.append(await self.run_test("Execute Actions", self._execute_actions))

        # 8. Verify checkpoints
        results.append(await self.run_test("Verify Checkpoints", self._verify_checkpoints))

        # 9. Cleanup
        if not self._no_cleanup and self._created_pod:
            results.append(await self.run_test("Cleanup", self._cleanup))

        # Close SSH connections
        await self._close_ssh()

        return self.make_result(results)

    # ------------------------------------------------------------------
    # Steps
    # ------------------------------------------------------------------

    async def _load_template(self):
        if self._template_path:
            self._template = parse_template_file(self._template_path)
        elif self._template_data:
            from .parser import parse_template_api
            self._template = parse_template_api(self._template_data)
        else:
            # Try to fetch from API by name
            assert self.api is not None, "No template path or API client provided"
            labs = await self.api.list_labs()
            lab_name = self.config.test_lab_template
            for lab in labs:
                if lab.get("name") == lab_name:
                    lab_data = await self.api.get_lab(lab["id"])
                    from .parser import parse_template_api
                    self._template = parse_template_api(lab_data)
                    break
            assert self._template is not None, f"Lab template '{lab_name}' not found"

        logger.info(
            "Loaded template: %s (%d objectives, %d VMs)",
            self._template.name,
            len(self._template.objectives),
            len(self._template.vms),
        )

    async def _load_scenario(self):
        self._scenario = load_scenario(self._scenario_path)
        logger.info(
            "Loaded scenario with %d steps", len(self._scenario.steps)
        )

    async def _generate_plan(self):
        assert self._template is not None
        self._action_plan = []

        for obj in self._template.objectives_sorted():
            # Check scenario override first
            step = self._scenario.get_step(obj.id) if self._scenario else None

            if step and step.actions:
                actions = step.actions
                target = step.target or _default_target(obj, self._template.vms)
                delay = step.delay_after
                source = "scenario"
            else:
                # Auto-generate from triggers
                actions = []
                target = _default_target(obj, self._template.vms)
                delay = 1.0
                source = "auto"

                for trigger in obj.triggers:
                    target = trigger.target or target
                    cmds = generate_actions(trigger, hint=obj.hint)
                    actions.extend(cmds)

            self._action_plan.append({
                "objective_id": obj.id,
                "description": obj.description,
                "points": obj.points,
                "target_vm": target,
                "actions": actions,
                "delay_after": delay,
                "source": source,
            })

        total_actions = sum(len(s["actions"]) for s in self._action_plan)
        logger.info("Generated %d actions for %d objectives", total_actions, len(self._action_plan))

    async def _display_plan(self):
        """Print the action plan (for dry-run mode)."""
        assert self._template is not None
        print(f"\n{'='*60}")
        print(f"  Simulation Plan: {self._template.name}")
        print(f"  Total Points: {self._template.total_points}")
        print(f"  Pass Threshold: {self._template.pass_threshold}%")
        print(f"{'='*60}\n")

        for entry in self._action_plan:
            src_tag = f"[{entry['source']}]"
            print(f"  Objective: {entry['objective_id']} ({entry['points']}pts) {src_tag}")
            print(f"  Description: {entry['description']}")
            print(f"  Target VM: {entry['target_vm']}")
            if entry["actions"]:
                print("  Commands:")
                for cmd in entry["actions"]:
                    print(f"    $ {cmd}")
            else:
                print("  Commands: (none generated - may need scenario override)")
            print()

        total_cmds = sum(len(e["actions"]) for e in self._action_plan)
        skipped = sum(1 for e in self._action_plan if not e["actions"])
        print(f"  Total: {total_cmds} commands across {len(self._action_plan)} objectives")
        if skipped:
            print(f"  Warning: {skipped} objective(s) have no generated commands")
        print()

    async def _provision_pod(self):
        if self._pod_id:
            # Use existing pod, get VM IPs
            pod = await self.api.get_pod(self._pod_id)
            assert pod.status == "running", f"Pod is not running: {pod.status}"
            for vm in pod.vms:
                if vm.ip_address:
                    self._vm_ip_map[vm.name] = vm.ip_address
            return

        # Create a new pod
        assert self.api is not None
        resp = await self.api.create_pod(
            lab_template=self._template.name,
            owner_id=self.config.test_user_id,
        )
        self._pod_id = resp.id
        self._created_pod = True

        # Wait for ready
        pod = await self.api.wait_for_pod_ready(
            self._pod_id,
            timeout=self.config.pod_provision_timeout,
        )
        for vm in pod.vms:
            if vm.ip_address:
                self._vm_ip_map[vm.name] = vm.ip_address

        assert self._vm_ip_map, "No VM IPs found after pod provisioning"

    async def _create_session(self):
        assert self.api is not None
        resp = await self.api.create_session(
            pod_id=self._pod_id,
            user_id=self.config.test_user_id,
            lab_template=self._template.name,
        )
        self._session_id = resp.session_id
        logger.info("Created session: %s", self._session_id)

    async def _connect_ssh(self):
        for vm_name, ip in self._vm_ip_map.items():
            client = SSHClient(
                host=ip,
                username=self._ssh_user,
                password=self._ssh_password,
                jump_host=self.config.infra_host,
                jump_user=self._ssh_user,
                jump_password=self._ssh_password,
            )
            # Open connection via AsyncExitStack for proper cleanup
            await self._exit_stack.enter_async_context(client.connect())
            self._vm_ssh_clients[vm_name] = client

        # Verify connectivity
        for vm_name, client in self._vm_ssh_clients.items():
            ok = await client.ping()
            assert ok, f"SSH ping failed for VM: {vm_name}"

    async def _execute_actions(self):
        assert self._template is not None
        failed_objectives: list[str] = []

        for entry in self._action_plan:
            obj_id = entry["objective_id"]
            target = entry["target_vm"]
            actions = entry["actions"]

            if not actions:
                logger.warning("No actions for objective: %s", obj_id)
                continue

            client = self._vm_ssh_clients.get(target)
            if not client:
                # Fall back to first available client
                if self._vm_ssh_clients:
                    client = next(iter(self._vm_ssh_clients.values()))
                else:
                    failed_objectives.append(f"{obj_id}: no SSH client")
                    continue

            logger.info("Executing %d commands for %s on %s", len(actions), obj_id, target)
            for cmd in actions:
                result = await client.run(cmd)
                if not result.success:
                    logger.warning(
                        "Command failed for %s: %s (exit=%d, stderr=%s)",
                        obj_id, cmd, result.exit_code, result.stderr,
                    )

            delay = entry.get("delay_after", 1.0)
            if delay > 0:
                await asyncio.sleep(delay)

        if failed_objectives:
            logger.warning("Failed objectives: %s", failed_objectives)

    async def _verify_checkpoints(self):
        assert self.api is not None and self._session_id is not None
        assert self._template is not None

        poll_interval = 5
        elapsed = 0
        expected_ids = {obj.id for obj in self._template.objectives}

        while elapsed < self._timeout:
            progress = await self.api.get_progress(self._session_id)
            passed_ids = {
                cp.id
                for cp in progress.checkpoints
                if cp.status == "passed"
            }

            if expected_ids <= passed_ids:
                # All passed
                total_pts = sum(
                    obj.points for obj in self._template.objectives
                    if obj.id in passed_ids
                )
                print(f"\n  All {len(expected_ids)} objectives passed! ({total_pts}/{self._template.total_points} points)")
                return

            await asyncio.sleep(poll_interval)
            elapsed += poll_interval

        # Timeout: report what passed
        progress = await self.api.get_progress(self._session_id)
        passed_ids = {
            cp.id
            for cp in progress.checkpoints
            if cp.status == "passed"
        }
        missing = expected_ids - passed_ids
        earned = sum(
            obj.points for obj in self._template.objectives
            if obj.id in passed_ids
        )

        msg = (
            f"Timeout after {self._timeout}s. "
            f"Passed: {len(passed_ids)}/{len(expected_ids)} objectives "
            f"({earned}/{self._template.total_points} points). "
            f"Missing: {', '.join(sorted(missing))}"
        )
        raise AssertionError(msg)

    async def _cleanup(self):
        if self._pod_id and self._created_pod:
            await self.api.delete_pod(self._pod_id)
            logger.info("Deleted pod: %s", self._pod_id)

    async def _close_ssh(self):
        await self._exit_stack.aclose()
        self._vm_ssh_clients.clear()


class MultiStudentSimulator:
    """Run multiple simulator instances concurrently."""

    def __init__(
        self,
        config: LabtestConfig,
        api: APIClient,
        num_students: int,
        **kwargs,
    ):
        self.config = config
        self.api = api
        self.num_students = num_students
        self._kwargs = kwargs

    async def run(self) -> list[TestSuiteResult]:
        """Run simulations for all students concurrently."""
        tasks = []
        for i in range(self.num_students):
            suite = SimulatorTestSuite(
                self.config,
                api=self.api,
                **self._kwargs,
            )
            suite.name = f"Lab Simulator (Student {i + 1})"
            tasks.append(suite.run())

        return await asyncio.gather(*tasks)


def _default_target(obj: Objective, vms: list[VMSpec]) -> str:
    """Get the default target VM for an objective."""
    if obj.triggers:
        return obj.triggers[0].target
    if vms:
        return vms[0].name
    return "unknown"
