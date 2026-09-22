"""TLS/SSL security checks via testssl.sh Docker image."""

from __future__ import annotations

import json
from pathlib import Path
from tempfile import TemporaryDirectory

from ..config import SectestConfig
from ..docker_runner import DockerRunner
from ..models import Finding, ScanResult, ScanStatus, Severity
from .base import BaseScanner

_TESTSSL_SEVERITY_MAP = {
    "CRITICAL": Severity.CRITICAL,
    "HIGH": Severity.HIGH,
    "MEDIUM": Severity.MEDIUM,
    "LOW": Severity.LOW,
    "INFO": Severity.INFO,
    "OK": Severity.INFO,
    "WARN": Severity.MEDIUM,
    "NOT ok": Severity.HIGH,
}


class SslScanner(BaseScanner):
    name = "ssl"
    docker_image = "drwetter/testssl.sh:latest"

    def __init__(self, config: SectestConfig, runner: DockerRunner):
        super().__init__(config, runner)

    def scan(self) -> ScanResult:
        """Run testssl.sh against the target if HTTPS is available."""
        # Only meaningful for HTTPS targets
        target = f"{self.config.target_host}:{self.config.target_port}"

        with TemporaryDirectory() as tmpdir:
            cmd = [
                "--jsonfile", "/output/ssl-results.json",
                "--severity", "LOW",
                "--fast",
                "--quiet",
                "--color", "0",
                target,
            ]

            exit_code, stdout, stderr = self.runner.run(
                self.docker_image,
                cmd,
                volumes={tmpdir: "/output"},
                timeout=self.config.scan_timeout,
            )

            findings: list[Finding] = []
            raw = stdout

            results_path = Path(tmpdir) / "ssl-results.json"
            if results_path.exists():
                raw = results_path.read_text()
                findings = self._parse_json(raw)

                out = self.config.output_path / "ssl-results.json"
                out.write_text(raw)

            if exit_code != 0 and not findings:
                # testssl may return non-zero if no TLS available — that's a finding itself
                if "doesn't seem to be a TLS" in (stdout + stderr):
                    findings.append(Finding(
                        title="No TLS/SSL configured",
                        severity=Severity.MEDIUM,
                        scanner=self.name,
                        description=f"Target {target} does not appear to have TLS/SSL enabled",
                        remediation="Configure TLS with a valid certificate",
                    ))

            status = ScanStatus.FINDINGS if findings else ScanStatus.PASSED
            if exit_code != 0 and not findings and "error" in stderr.lower():
                status = ScanStatus.ERROR

            return ScanResult(
                scanner=self.name,
                status=status,
                findings=findings,
                raw_output=raw[:5000],
                error=stderr[:2000] if status == ScanStatus.ERROR else "",
            )

    @staticmethod
    def _parse_json(raw: str) -> list[Finding]:
        """Parse testssl.sh JSON output."""
        findings: list[Finding] = []
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            return findings

        entries = data if isinstance(data, list) else data.get("scanResult", [])
        for entry in entries:
            if isinstance(entry, dict) and "id" in entry:
                sev_str = entry.get("severity", "INFO").upper()
                severity = _TESTSSL_SEVERITY_MAP.get(sev_str, Severity.INFO)

                # Skip pure INFO/OK entries
                if severity == Severity.INFO:
                    continue

                findings.append(Finding(
                    title=entry.get("id", "unknown"),
                    severity=severity,
                    scanner="ssl",
                    description=entry.get("finding", ""),
                    evidence=entry.get("finding", ""),
                    metadata={
                        "ip": entry.get("ip", ""),
                        "port": entry.get("port", ""),
                        "cve": entry.get("cve", ""),
                    },
                ))

        return findings
