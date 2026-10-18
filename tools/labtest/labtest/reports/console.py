"""Rich console reporter for test results."""

from __future__ import annotations
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

from ..models.results import TestReport, TestResult, TestStatus, TestSuiteResult


class ConsoleReporter:
    """Reporter that outputs test results to console using Rich.

    Usage:
        reporter = ConsoleReporter()
        reporter.report_suite(suite_result)
        reporter.report(full_report)
    """

    def __init__(self, console: Console | None = None):
        """Initialize reporter.

        Args:
            console: Rich Console instance (creates new one if not provided)
        """
        self.console = console or Console()

    def _status_style(self, status: TestStatus) -> Text:
        """Get styled text for test status."""
        styles = {
            TestStatus.PASSED: ("PASS", "bold green"),
            TestStatus.FAILED: ("FAIL", "bold red"),
            TestStatus.SKIPPED: ("SKIP", "bold yellow"),
            TestStatus.ERROR: ("ERROR", "bold red on white"),
        }
        text, style = styles.get(status, (status.value.upper(), ""))
        return Text(text, style=style)

    def _format_duration(self, seconds: float) -> str:
        """Format duration in human-readable form."""
        if seconds < 0.001:
            return "<1ms"
        elif seconds < 1:
            return f"{seconds * 1000:.0f}ms"
        elif seconds < 60:
            return f"{seconds:.2f}s"
        else:
            mins = int(seconds // 60)
            secs = seconds % 60
            return f"{mins}m {secs:.1f}s"

    def _truncate_message(self, message: str, max_length: int = 60) -> str:
        """Truncate message with ellipsis if too long."""
        if len(message) <= max_length:
            return message
        return message[: max_length - 3] + "..."

    def report_result(self, result: TestResult) -> None:
        """Print a single test result.

        Args:
            result: Test result to print
        """
        status = self._status_style(result.status)
        duration = self._format_duration(result.duration)
        message = self._truncate_message(result.message) if result.message else ""

        self.console.print(f"  {status} {result.name} [{duration}] {message}")

    def report_suite(self, suite: TestSuiteResult) -> None:
        """Print a test suite result as a table.

        Args:
            suite: Test suite result to print
        """
        # Create table
        table = Table(
            title=suite.name,
            show_header=True,
            header_style="bold magenta",
            border_style="dim",
            title_style="bold blue",
        )

        table.add_column("Test", style="cyan", no_wrap=True, min_width=30)
        table.add_column("Status", justify="center", width=8)
        table.add_column("Duration", justify="right", width=10)
        table.add_column("Message", style="dim", max_width=50)

        for result in suite.results:
            table.add_row(
                result.name,
                self._status_style(result.status),
                self._format_duration(result.duration),
                self._truncate_message(result.message, 50),
            )

        self.console.print()
        self.console.print(table)

        # Summary line
        summary_parts = [
            f"[bold]Total: {suite.total}[/]",
            f"[green]Passed: {suite.passed}[/]",
        ]
        if suite.failed > 0:
            summary_parts.append(f"[red]Failed: {suite.failed}[/]")
        if suite.errors > 0:
            summary_parts.append(f"[red]Errors: {suite.errors}[/]")
        if suite.skipped > 0:
            summary_parts.append(f"[yellow]Skipped: {suite.skipped}[/]")
        summary_parts.append(f"[dim]Duration: {self._format_duration(suite.duration)}[/]")

        self.console.print("  " + " | ".join(summary_parts))

    def report(self, report: TestReport) -> None:
        """Print a full test report with all suites.

        Args:
            report: Test report to print
        """
        for suite in report.suites:
            self.report_suite(suite)

        self.console.print()

        # Overall summary
        if report.success:
            status_text = "[bold green]ALL TESTS PASSED[/]"
            border_style = "green"
        else:
            status_text = "[bold red]TESTS FAILED[/]"
            border_style = "red"

        summary = (
            f"{status_text}\n\n"
            f"Suites: {len(report.suites)} | "
            f"Tests: {report.total} | "
            f"[green]Passed: {report.passed}[/] | "
            f"[red]Failed: {report.failed}[/] | "
            f"[yellow]Skipped: {report.skipped}[/]\n"
            f"Total Duration: {self._format_duration(report.duration)}"
        )

        self.console.print(Panel(summary, title="Summary", border_style=border_style))

    def print_header(self, title: str) -> None:
        """Print a section header.

        Args:
            title: Header title
        """
        self.console.print()
        self.console.rule(f"[bold blue]{title}[/]")

    def print_progress(self, message: str) -> None:
        """Print a progress message.

        Args:
            message: Progress message
        """
        self.console.print(f"[dim]>>> {message}[/]")

    def print_error(self, message: str) -> None:
        """Print an error message.

        Args:
            message: Error message
        """
        self.console.print(f"[bold red]ERROR:[/] {message}")

    def print_success(self, message: str) -> None:
        """Print a success message.

        Args:
            message: Success message
        """
        self.console.print(f"[bold green]SUCCESS:[/] {message}")

    def print_warning(self, message: str) -> None:
        """Print a warning message.

        Args:
            message: Warning message
        """
        self.console.print(f"[bold yellow]WARNING:[/] {message}")

    def print_diagnostic(self, name: str, status: bool, details: str = "") -> None:
        """Print a diagnostic check result.

        Args:
            name: Check name
            status: True if check passed
            details: Optional details
        """
        icon = "[green][/]" if status else "[red][/]"
        detail_text = f" - {details}" if details else ""
        self.console.print(f"  {icon} {name}{detail_text}")
