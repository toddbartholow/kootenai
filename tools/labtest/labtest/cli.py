"""Command-line interface for labtest."""

from __future__ import annotations
import asyncio
import logging
import sys
from pathlib import Path
from typing import Optional

import click
from rich.console import Console

from .config import LabtestConfig
from .clients.api import APIClient
from .clients.database import DatabaseClient
from .clients.nats import NATSClient
from .clients.ssh import SSHClient
from .models.results import TestReport
from .reports.console import ConsoleReporter
from .reports.json import JSONReporter
from .simulator.runner import MultiStudentSimulator, SimulatorTestSuite
from .tests.auth import AdminAuthTestSuite, AuthTestSuite
from .tests.checkpoints import CheckpointTestSuite
from .tests.e2e import E2ETestSuite, QuickE2ETestSuite
from .tests.health import HealthTestSuite, QuickHealthSuite
from .tests.pods import PodTestSuite
from .tests.sessions import SessionTestSuite

console = Console()
reporter = ConsoleReporter(console)


def setup_logging(verbose: bool):
    """Configure logging based on verbosity."""
    level = logging.DEBUG if verbose else logging.WARNING
    logging.basicConfig(
        level=level,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )


@click.group()
@click.option("--config", "-c", type=click.Path(exists=True), help="Config file path")
@click.option("--verbose", "-v", is_flag=True, help="Verbose output")
@click.option("--json", "json_output", is_flag=True, help="Output as JSON")
@click.option("--api-url", envvar="LABTEST_API_URL", help="API base URL")
@click.pass_context
def cli(ctx, config, verbose, json_output, api_url):
    """Kootenai Platform E2E Testing CLI.

    Run tests against the Kootenai platform to verify functionality.
    """
    ctx.ensure_object(dict)

    # Load config
    config_path = Path(config) if config else None
    overrides = {}
    if api_url:
        overrides["api_base_url"] = api_url
    if verbose:
        overrides["verbose"] = True
    if json_output:
        overrides["json_output"] = True

    ctx.obj["config"] = LabtestConfig.load(config_path, **overrides)
    setup_logging(ctx.obj["config"].verbose)


@cli.command()
@click.pass_context
def quick(ctx):
    """Run quick health checks (< 10 seconds).

    Tests basic connectivity to all services.
    """
    asyncio.run(_run_quick(ctx.obj["config"]))


async def _run_quick(config: LabtestConfig):
    """Run quick health checks."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    report = TestReport()

    try:
        async with api.session():
            # API quick check
            suite = QuickHealthSuite(config, api=api)
            result = await suite.run()
            report.suites.append(result)
    except Exception as e:
        console.print(f"[red]Connection error: {e}[/]")
        sys.exit(1)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.pass_context
def health(ctx):
    """Run health and connectivity tests.

    Tests API, database, and NATS connectivity.
    """
    asyncio.run(_run_health(ctx.obj["config"]))


async def _run_health(config: LabtestConfig):
    """Run full health tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    db = DatabaseClient(config.db_dsn)
    nats = NATSClient(config.nats_url)

    report = TestReport()

    try:
        async with api.session():
            async with db.session():
                async with nats.session():
                    suite = HealthTestSuite(config, api=api, db=db, nats=nats)
                    result = await suite.run()
                    report.suites.append(result)
    except Exception as e:
        console.print(f"[red]Connection error: {e}[/]")
        sys.exit(1)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.option("--admin", is_flag=True, help="Include admin auth tests")
@click.pass_context
def auth(ctx, admin):
    """Run authentication tests.

    Tests login, token validation, and password management.
    """
    asyncio.run(_run_auth(ctx.obj["config"], admin))


async def _run_auth(config: LabtestConfig, include_admin: bool):
    """Run authentication tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    report = TestReport()

    async with api.session():
        # Basic auth tests
        suite = AuthTestSuite(config, api=api)
        result = await suite.run()
        report.suites.append(result)

        # Admin auth tests (if requested and admin creds configured)
        if include_admin and config.admin_email and config.admin_password:
            suite = AdminAuthTestSuite(config, api=api)
            result = await suite.run()
            report.suites.append(result)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.pass_context
def pods(ctx):
    """Run pod lifecycle tests.

    Creates a pod, waits for it to be ready, then cleans up.
    """
    asyncio.run(_run_pods(ctx.obj["config"]))


async def _run_pods(config: LabtestConfig):
    """Run pod lifecycle tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    report = TestReport()

    async with api.session():
        suite = PodTestSuite(config, api=api)
        result = await suite.run()
        report.suites.append(result)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.pass_context
def sessions(ctx):
    """Run session management tests.

    Tests session creation, progress tracking, and submission.
    """
    asyncio.run(_run_sessions(ctx.obj["config"]))


async def _run_sessions(config: LabtestConfig):
    """Run session tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    report = TestReport()

    async with api.session():
        suite = SessionTestSuite(config, api=api)
        result = await suite.run()
        report.suites.append(result)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.option("--pod-id", help="Pod ID to use")
@click.option("--vm-ip", help="VM IP address for SSH")
@click.pass_context
def checkpoints(ctx, pod_id, vm_ip):
    """Run checkpoint detection tests.

    Tests that lab objectives trigger checkpoint completion.
    """
    asyncio.run(_run_checkpoints(ctx.obj["config"], pod_id, vm_ip))


async def _run_checkpoints(config: LabtestConfig, pod_id: Optional[str], vm_ip: Optional[str]):
    """Run checkpoint tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    ssh = None

    # Setup SSH client if we have a VM IP
    if vm_ip:
        ssh = SSHClient(
            host=vm_ip,
            username=config.vm_ssh_user,
            password=config.vm_ssh_password,
            jump_host=config.infra_host,
            jump_user=config.vm_ssh_user,
            jump_password=config.vm_ssh_password,
        )

    report = TestReport()

    async with api.session():
        if ssh:
            async with ssh.connect():
                suite = CheckpointTestSuite(
                    config, api=api, ssh=ssh, pod_id=pod_id, vm_ip=vm_ip
                )
                result = await suite.run()
                report.suites.append(result)
        else:
            suite = CheckpointTestSuite(config, api=api, pod_id=pod_id, vm_ip=vm_ip)
            result = await suite.run()
            report.suites.append(result)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.option("--lab", "-l", help="Lab template name")
@click.option("--quick", "quick_mode", is_flag=True, help="Use existing pod (faster)")
@click.option("--no-cleanup", is_flag=True, help="Don't delete pod after test")
@click.pass_context
def e2e(ctx, lab, quick_mode, no_cleanup):
    """Run full end-to-end lab lifecycle test.

    Creates a pod, completes objectives, verifies checkpoints, and submits.
    """
    config = ctx.obj["config"]
    if lab:
        config.test_lab_template = lab
    if no_cleanup:
        config.cleanup_on_success = False

    asyncio.run(_run_e2e(config, quick_mode))


async def _run_e2e(config: LabtestConfig, quick_mode: bool):
    """Run E2E tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    report = TestReport()

    async with api.session():
        if quick_mode:
            # Find existing pod and get its VM IP
            pods_resp = await api.list_pods()
            vm_ip = None
            for pod in pods_resp.pods:
                if pod.status == "running" and pod.vms and pod.vms[0].ip_address:
                    vm_ip = pod.vms[0].ip_address
                    break

            ssh = None
            if vm_ip:
                ssh = SSHClient(
                    host=vm_ip,
                    username=config.vm_ssh_user,
                    password=config.vm_ssh_password,
                    jump_host=config.infra_host,
                    jump_user=config.vm_ssh_user,
                    jump_password=config.vm_ssh_password,
                )

            if ssh:
                async with ssh.connect():
                    suite = QuickE2ETestSuite(config, api=api, ssh=ssh)
                    result = await suite.run()
                    report.suites.append(result)
            else:
                suite = QuickE2ETestSuite(config, api=api)
                result = await suite.run()
                report.suites.append(result)
        else:
            # Full E2E - will create SSH client after pod is ready
            suite = E2ETestSuite(config, api=api)
            result = await suite.run()
            report.suites.append(result)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command("all")
@click.option("--quick", "quick_only", is_flag=True, help="Only quick tests")
@click.option("--full", "full_only", is_flag=True, help="Only full tests")
@click.pass_context
def run_all(ctx, quick_only, full_only):
    """Run all test suites.

    By default runs both quick and full tests.
    """
    config = ctx.obj["config"]
    if quick_only:
        config.run_quick_tests = True
        config.run_full_tests = False
    elif full_only:
        config.run_quick_tests = False
        config.run_full_tests = True

    asyncio.run(_run_all(config))


async def _run_all(config: LabtestConfig):
    """Run all tests."""
    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    db = DatabaseClient(config.db_dsn)
    nats = NATSClient(config.nats_url)

    report = TestReport()

    try:
        async with api.session():
            async with db.session():
                async with nats.session():
                    # Quick tests
                    if config.run_quick_tests:
                        reporter.print_header("Quick Tests")
                        suite = HealthTestSuite(config, api=api, db=db, nats=nats)
                        result = await suite.run()
                        report.suites.append(result)

                        reporter.print_header("Authentication Tests")
                        suite = AuthTestSuite(config, api=api)
                        result = await suite.run()
                        report.suites.append(result)

                    # Full tests
                    if config.run_full_tests:
                        reporter.print_header("Pod Lifecycle Tests")
                        suite = PodTestSuite(config, api=api)
                        result = await suite.run()
                        report.suites.append(result)

                        reporter.print_header("Session Tests")
                        suite = SessionTestSuite(config, api=api)
                        result = await suite.run()
                        report.suites.append(result)

    except Exception as e:
        console.print(f"[red]Error: {e}[/]")
        sys.exit(1)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


@cli.command()
@click.pass_context
def diagnose(ctx):
    """Quick diagnostic check of all services.

    Shows connection status for each service.
    """
    asyncio.run(_run_diagnose(ctx.obj["config"]))


async def _run_diagnose(config: LabtestConfig):
    """Run diagnostic checks."""
    console.print("\n[bold]Kootenai Platform Diagnostics[/]\n")

    # API
    console.print("[bold]API Server[/]")
    api = APIClient(config.api_url, config.api_timeout)
    try:
        async with api.session():
            health = await api.health()
            reporter.print_diagnostic("API Health", health.status == "ok", config.api_url)

            version = await api.version()
            reporter.print_diagnostic(
                "API Version",
                True,
                f"{version.version} ({version.commit[:8]})",
            )
    except Exception as e:
        reporter.print_diagnostic("API Connection", False, str(e))

    # Database
    console.print("\n[bold]Database[/]")
    db = DatabaseClient(config.db_dsn)
    try:
        async with db.session():
            ping = await db.ping()
            reporter.print_diagnostic("Database Ping", ping, f"{config.db_host}:{config.db_port}")

            tables = await db.check_tables_exist()
            all_exist = all(tables.values())
            reporter.print_diagnostic(
                "Tables Exist",
                all_exist,
                f"{sum(tables.values())}/{len(tables)} tables",
            )
    except Exception as e:
        reporter.print_diagnostic("Database Connection", False, str(e))

    # NATS
    console.print("\n[bold]NATS Messaging[/]")
    nats = NATSClient(config.nats_url)
    try:
        async with nats.session():
            connected = await nats.is_connected()
            reporter.print_diagnostic("NATS Connection", connected, config.nats_url)

            stream = await nats.get_stream_info("LABS")
            has_stream = stream is not None
            msg_count = stream.get("messages", 0) if stream else 0
            reporter.print_diagnostic(
                "LABS Stream",
                has_stream,
                f"{msg_count} messages" if has_stream else "Not found",
            )
    except Exception as e:
        reporter.print_diagnostic("NATS Connection", False, str(e))

    console.print()


@cli.command("list-labs")
@click.pass_context
def list_labs(ctx):
    """List available lab templates."""
    asyncio.run(_list_labs(ctx.obj["config"]))


async def _list_labs(config: LabtestConfig):
    """List lab templates."""
    api = APIClient(config.api_url, config.api_timeout)

    async with api.session():
        labs = await api.list_labs()

        if not labs:
            console.print("[yellow]No lab templates found[/]")
            return

        console.print("\n[bold]Available Lab Templates[/]\n")
        for lab in labs:
            name = lab.get("name", "Unknown")
            lab_id = lab.get("id", "")[:8]
            platform = lab.get("platform", "unknown")
            console.print(f"  - {name} [dim]({lab_id}...)[/] [{platform}]")
        console.print()


@cli.command()
@click.option("--lab", "-l", required=True, help="Template YAML path or lab name")
@click.option("--scenario", "-s", type=click.Path(exists=True), help="Scenario YAML override")
@click.option("--students", "-n", default=1, type=int, help="Number of concurrent students")
@click.option("--pod-id", help="Use existing pod (skip provisioning)")
@click.option("--dry-run", is_flag=True, help="Show generated commands without executing")
@click.option("--no-cleanup", is_flag=True, help="Keep pods after simulation")
@click.option("--timeout", default=120, type=int, help="Checkpoint detection timeout (seconds)")
@click.option("--ssh-user", help="VM SSH username")
@click.option("--ssh-pass", help="VM SSH password")
@click.pass_context
def simulate(ctx, lab, scenario, students, pod_id, dry_run, no_cleanup, timeout, ssh_user, ssh_pass):
    """Simulate student actions and verify checkpoint triggers.

    Auto-generates SSH commands from lab template trigger definitions,
    executes them on VMs, and verifies all checkpoints pass.

    Examples:

        labtest simulate --lab templates/examples/simple-linux-intro.yaml --dry-run

        labtest simulate --lab "Simple Linux Introduction"

        labtest simulate --lab templates/examples/simple-linux-intro.yaml --students 3
    """
    config = ctx.obj["config"]
    if no_cleanup:
        config.cleanup_on_success = False

    asyncio.run(_run_simulate(
        config,
        lab=lab,
        scenario_path=scenario,
        num_students=students,
        pod_id=pod_id,
        dry_run=dry_run,
        no_cleanup=no_cleanup,
        timeout=timeout,
        ssh_user=ssh_user,
        ssh_pass=ssh_pass,
    ))


async def _run_simulate(
    config: LabtestConfig,
    *,
    lab: str,
    scenario_path: Optional[str],
    num_students: int,
    pod_id: Optional[str],
    dry_run: bool,
    no_cleanup: bool,
    timeout: int,
    ssh_user: Optional[str],
    ssh_pass: Optional[str],
):
    """Run lab simulation."""
    # Determine if lab is a file path or a lab name
    template_path = None
    lab_path = Path(lab)
    if lab_path.exists() and lab_path.suffix in (".yaml", ".yml"):
        template_path = str(lab_path)
    else:
        config.test_lab_template = lab

    common_kwargs = dict(
        template_path=template_path,
        scenario_path=scenario_path,
        pod_id=pod_id,
        dry_run=dry_run,
        no_cleanup=no_cleanup,
        ssh_user=ssh_user,
        ssh_password=ssh_pass,
        timeout=timeout,
    )

    api = APIClient(config.api_url, config.api_timeout, config.verbose)
    report = TestReport()

    try:
        async with api.session():
            if num_students > 1 and not dry_run:
                multi = MultiStudentSimulator(
                    config, api, num_students, **common_kwargs
                )
                results = await multi.run()
                report.suites.extend(results)
            else:
                suite = SimulatorTestSuite(config, api=api, **common_kwargs)
                result = await suite.run()
                report.suites.append(result)
    except Exception as e:
        console.print(f"[red]Error: {e}[/]")
        sys.exit(1)

    if config.json_output:
        print(JSONReporter().to_json(report))
    else:
        reporter.report(report)

    sys.exit(0 if report.success else 1)


def main():
    """Main entry point."""
    cli(obj={})


if __name__ == "__main__":
    main()
