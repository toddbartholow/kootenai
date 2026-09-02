"""Security testing CLI for Kootenai platform.

Orchestrates Docker-based security scanners (nmap, nuclei, ZAP, testssl.sh)
and native HTTP security checks against a target host.

Usage:
    sectest full              # Run all scanners
    sectest quick             # HTTP checks only (no Docker)
    sectest scan nmap         # Run specific scanner
    sectest scan nuclei       # Nuclei vulnerability scan
    sectest scan zap          # OWASP ZAP API scan
    sectest scan ssl          # TLS/SSL check
"""

from __future__ import annotations

import time
from pathlib import Path
from typing import Optional

import click
from rich.console import Console

from .config import SectestConfig
from .docker_runner import DockerRunner
from .models import PentestReport, ScanResult, ScanStatus
from .reports.console import (
    print_findings_table,
    print_scan_header,
    print_scanner_done,
    print_scanner_start,
    print_summary,
)
from .reports.json_report import export_json
from .scanners.http_checks import HttpSecurityChecker
from .scanners.nmap_scan import NmapScanner
from .scanners.nuclei_scan import NucleiScanner
from .scanners.ssl_scan import SslScanner
from .scanners.zap_scan import ZapScanner

console = Console()

SCANNER_MAP = {
    "nmap": NmapScanner,
    "nuclei": NucleiScanner,
    "zap": ZapScanner,
    "ssl": SslScanner,
    "http": HttpSecurityChecker,
}


@click.group()
@click.option("--config", "-c", type=click.Path(exists=True), help="Config file path")
@click.option("--verbose", "-v", is_flag=True, help="Verbose output")
@click.option("--target", "-t", envvar="SECTEST_TARGET_HOST", help="Target host")
@click.option("--port", "-p", type=int, envvar="SECTEST_TARGET_PORT", help="Target port")
@click.option("--output", "-o", type=click.Path(), help="Output directory")
@click.pass_context
def cli(ctx, config, verbose, target, port, output):
    """Kootenai Security Testing CLI.

    Orchestrates Docker-based security scanners against the target infrastructure.
    """
    ctx.ensure_object(dict)

    overrides = {}
    if verbose:
        overrides["verbose"] = verbose
    if target:
        overrides["target_host"] = target
    if port:
        overrides["target_port"] = port
    if output:
        overrides["output_dir"] = output

    config_path = Path(config) if config else None
    ctx.obj["config"] = SectestConfig.load(config_path, **overrides)


@cli.command()
@click.pass_context
def full(ctx):
    """Run all security scanners (nmap, nuclei, ZAP, SSL, HTTP checks)."""
    config: SectestConfig = ctx.obj["config"]
    runner = DockerRunner(config)

    if not runner.is_available():
        console.print("[red]Docker is not available. Install Docker to run full scans.[/]")
        console.print("Use [cyan]sectest quick[/] for HTTP-only checks (no Docker needed).")
        raise SystemExit(1)

    print_scan_header(config.target_url)

    # Pull all images first
    console.print("[bold]Pulling scanner images...[/]")
    images = [
        NmapScanner.docker_image,
        NucleiScanner.docker_image,
        ZapScanner.docker_image,
        SslScanner.docker_image,
    ]
    avail = runner.ensure_images(images)

    report = PentestReport(target=config.target_url)
    start = time.time()

    # Run scanners in sequence (they're I/O heavy, parallelism doesn't help much
    # with Docker containers competing for the same target)
    scanners = [
        ("nmap", NmapScanner),
        ("http-security", HttpSecurityChecker),
        ("nuclei", NucleiScanner),
        ("zap", ZapScanner),
        ("ssl", SslScanner),
    ]

    for name, cls in scanners:
        scanner = cls(config, runner)

        # Skip if Docker image unavailable
        if scanner.docker_image and not avail.get(scanner.docker_image, False):
            result = ScanResult(scanner=name, status=ScanStatus.SKIPPED, error="Image pull failed")
            report.scan_results.append(result)
            console.print(f"  [dim]{name}: SKIPPED (image unavailable)[/]")
            continue

        print_scanner_start(name)
        result = scanner.timed_scan()
        report.scan_results.append(result)
        print_scanner_done(result)

    report.duration = time.time() - start

    # Print detailed findings and summary
    print_findings_table(report.all_findings)
    print_summary(report)

    # Export JSON report
    json_path = export_json(report, config.output_path)
    console.print(f"JSON report: [cyan]{json_path}[/]")
    console.print(f"All results: [cyan]{config.output_path}[/]")


@cli.command()
@click.pass_context
def quick(ctx):
    """Run HTTP security checks only (no Docker required)."""
    config: SectestConfig = ctx.obj["config"]
    runner = DockerRunner(config)  # Not used but needed by interface

    print_scan_header(config.target_url)

    report = PentestReport(target=config.target_url)
    start = time.time()

    scanner = HttpSecurityChecker(config, runner)
    print_scanner_start("http-security")
    result = scanner.timed_scan()
    report.scan_results.append(result)
    print_scanner_done(result)

    report.duration = time.time() - start

    print_findings_table(report.all_findings)
    print_summary(report)

    json_path = export_json(report, config.output_path)
    console.print(f"JSON report: [cyan]{json_path}[/]")


@cli.command()
@click.argument("scanner_name", type=click.Choice(list(SCANNER_MAP.keys())))
@click.pass_context
def scan(ctx, scanner_name):
    """Run a specific scanner.

    SCANNER_NAME: nmap, nuclei, zap, ssl, or http
    """
    config: SectestConfig = ctx.obj["config"]
    runner = DockerRunner(config)

    cls = SCANNER_MAP[scanner_name]
    scanner = cls(config, runner)

    if scanner.docker_image:
        if not runner.is_available():
            console.print(f"[red]Docker required for {scanner_name} but not available.[/]")
            raise SystemExit(1)

        console.print(f"Pulling [cyan]{scanner.docker_image}[/]...")
        if not runner.pull_image(scanner.docker_image):
            console.print(f"[red]Failed to pull {scanner.docker_image}[/]")
            raise SystemExit(1)

    print_scan_header(config.target_url)

    report = PentestReport(target=config.target_url)
    start = time.time()

    print_scanner_start(scanner_name)
    result = scanner.timed_scan()
    report.scan_results.append(result)
    print_scanner_done(result)

    report.duration = time.time() - start

    print_findings_table(report.all_findings)
    print_summary(report)

    json_path = export_json(report, config.output_path)
    console.print(f"JSON report: [cyan]{json_path}[/]")


@cli.command()
@click.pass_context
def check(ctx):
    """Verify Docker and scanner availability."""
    config: SectestConfig = ctx.obj["config"]
    runner = DockerRunner(config)

    console.print("[bold]Environment Check[/]\n")
    console.print(f"  Target: [cyan]{config.target_url}[/]")
    console.print(f"  Output: [cyan]{config.output_dir}[/]")
    console.print()

    # Docker
    docker_ok = runner.is_available()
    icon = "[green]OK[/]" if docker_ok else "[red]MISSING[/]"
    console.print(f"  Docker: {icon}")

    if docker_ok:
        images = {
            "nmap": NmapScanner.docker_image,
            "nuclei": NucleiScanner.docker_image,
            "zap": ZapScanner.docker_image,
            "ssl": SslScanner.docker_image,
        }
        for name, img in images.items():
            from subprocess import run as sp_run
            result = sp_run(
                [config.docker_bin, "image", "inspect", img],
                capture_output=True,
            )
            present = result.returncode == 0
            icon = "[green]cached[/]" if present else "[yellow]needs pull[/]"
            console.print(f"    {name} ({img}): {icon}")

    console.print()

    # Target connectivity
    import httpx
    try:
        resp = httpx.get(config.target_url, timeout=5, follow_redirects=True)
        console.print(f"  Target reachable: [green]OK[/] (HTTP {resp.status_code})")
    except httpx.ConnectError:
        console.print(f"  Target reachable: [red]FAILED[/] (cannot connect)")
    except httpx.HTTPError as e:
        console.print(f"  Target reachable: [yellow]PARTIAL[/] ({e})")


if __name__ == "__main__":
    cli()
