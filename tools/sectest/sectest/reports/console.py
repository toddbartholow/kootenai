"""Rich console report output."""

from __future__ import annotations

from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

from ..models import Finding, PentestReport, ScanResult, ScanStatus, Severity

console = Console()

SEVERITY_COLORS = {
    Severity.CRITICAL: "bold red",
    Severity.HIGH: "red",
    Severity.MEDIUM: "yellow",
    Severity.LOW: "cyan",
    Severity.INFO: "dim",
}

STATUS_ICONS = {
    ScanStatus.PASSED: "[green]PASS[/]",
    ScanStatus.FINDINGS: "[yellow]FINDINGS[/]",
    ScanStatus.ERROR: "[red]ERROR[/]",
    ScanStatus.SKIPPED: "[dim]SKIP[/]",
}


def print_scan_header(target: str) -> None:
    console.print()
    console.print(Panel.fit(
        f"[bold]Security Scan: [cyan]{target}[/cyan][/bold]",
        border_style="blue",
    ))
    console.print()


def print_scanner_start(scanner_name: str) -> None:
    console.print(f"  [bold]{scanner_name}[/] scanning...", end="")


def print_scanner_done(result: ScanResult) -> None:
    status = STATUS_ICONS.get(result.status, str(result.status))
    count = len(result.findings)
    duration = f"{result.duration:.1f}s"

    parts = [f" {status}"]
    if count > 0:
        parts.append(f" ({count} findings)")
    parts.append(f" [{duration}]")

    console.print("".join(parts))

    if result.error:
        console.print(f"    [red]Error: {result.error[:200]}[/]")


def print_findings_table(findings: list[Finding]) -> None:
    if not findings:
        console.print("\n  [green]No findings.[/]\n")
        return

    table = Table(title="Findings", show_lines=True, expand=True)
    table.add_column("#", style="dim", width=3)
    table.add_column("Severity", width=10)
    table.add_column("Scanner", width=14)
    table.add_column("Title", min_width=30)
    table.add_column("Details", min_width=30)

    for i, f in enumerate(findings, 1):
        color = SEVERITY_COLORS.get(f.severity, "")
        sev_text = Text(f.severity.value.upper(), style=color)

        details = f.description[:80] if f.description else ""
        if f.evidence:
            details += f"\n[dim]{f.evidence[:80]}[/]"

        table.add_row(
            str(i),
            sev_text,
            f.scanner,
            f.title,
            details,
        )

    console.print()
    console.print(table)


def print_summary(report: PentestReport) -> None:
    console.print()
    counts = report.finding_counts
    total = len(report.all_findings)

    # Summary panel
    lines = [
        f"[bold]Target:[/] {report.target}",
        f"[bold]Duration:[/] {report.duration:.1f}s",
        f"[bold]Scanners:[/] {len(report.scan_results)}",
        f"[bold]Total findings:[/] {total}",
    ]

    if counts:
        severity_line = "  ".join(
            f"[{SEVERITY_COLORS.get(Severity(s), '')}]{s.upper()}: {c}[/]"
            for s, c in sorted(counts.items(), key=lambda x: Severity(x[0]).value)
        )
        lines.append(f"  {severity_line}")

    border = "red" if report.has_critical else "yellow" if report.has_high else "green"
    console.print(Panel("\n".join(lines), title="Summary", border_style=border))

    # Per-scanner summary
    table = Table(title="Scanner Summary")
    table.add_column("Scanner")
    table.add_column("Status")
    table.add_column("Findings")
    table.add_column("Duration")

    for sr in report.scan_results:
        status = STATUS_ICONS.get(sr.status, str(sr.status))
        fc = sr.finding_counts
        count_str = ", ".join(f"{s}: {c}" for s, c in fc.items()) if fc else "-"
        table.add_row(sr.scanner, status, count_str, f"{sr.duration:.1f}s")

    console.print(table)
    console.print()
