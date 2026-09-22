"""OWASP ZAP API scanner via Docker."""

from __future__ import annotations

import json
from pathlib import Path
from tempfile import TemporaryDirectory

from ..config import SectestConfig
from ..docker_runner import DockerRunner
from ..models import Finding, ScanResult, ScanStatus, Severity
from .base import BaseScanner

_ZAP_RISK_MAP = {
    0: Severity.INFO,
    1: Severity.LOW,
    2: Severity.MEDIUM,
    3: Severity.HIGH,
}


class ZapScanner(BaseScanner):
    name = "zap"
    docker_image = "ghcr.io/zaproxy/zaproxy:stable"

    def __init__(self, config: SectestConfig, runner: DockerRunner):
        super().__init__(config, runner)

    def scan(self) -> ScanResult:
        """Run ZAP API scan against the target."""
        with TemporaryDirectory() as tmpdir:
            # ZAP API scan mode
            cmd = [
                "zap-api-scan.py",
                "-t", self.config.target_api_url,
                "-f", "openapi",
                "-J", "zap-report.json",
                "-r", "zap-report.html",
                "-w", "zap-report.md",
                "-l", "WARN",
                "-z", "-config api.disablekey=true",
            ]

            if self.config.zap_ajax_spider:
                cmd.extend(["-j"])  # Enable AJAX spider

            exit_code, stdout, stderr = self.runner.run(
                self.docker_image,
                cmd,
                volumes={tmpdir: "/zap/wrk"},
                timeout=self.config.scan_timeout,
            )

            findings: list[Finding] = []
            raw = stdout

            # Parse JSON report
            report_path = Path(tmpdir) / "zap-report.json"
            if report_path.exists():
                raw_json = report_path.read_text()
                findings = self._parse_json_report(raw_json)

                # Copy reports to output dir
                for fname in ("zap-report.json", "zap-report.html", "zap-report.md"):
                    src = Path(tmpdir) / fname
                    if src.exists():
                        dst = self.config.output_path / fname
                        dst.write_text(src.read_text())

            # ZAP returns exit codes: 0=pass, 1=warnings, 2=failures, 3=errors
            if exit_code == 3 and not findings:
                status = ScanStatus.ERROR
            elif findings:
                status = ScanStatus.FINDINGS
            else:
                status = ScanStatus.PASSED

            return ScanResult(
                scanner=self.name,
                status=status,
                findings=findings,
                raw_output=raw[:5000],
                error=stderr[:2000] if exit_code == 3 else "",
            )

    @staticmethod
    def _parse_json_report(raw_json: str) -> list[Finding]:
        """Parse ZAP JSON report into findings."""
        findings: list[Finding] = []
        try:
            report = json.loads(raw_json)
        except json.JSONDecodeError:
            return findings

        for site in report.get("site", []):
            for alert in site.get("alerts", []):
                risk = int(alert.get("riskcode", 0))
                severity = _ZAP_RISK_MAP.get(risk, Severity.INFO)

                instances = alert.get("instances", [])
                evidence_parts = []
                for inst in instances[:3]:  # Limit to 3 instances
                    uri = inst.get("uri", "")
                    method = inst.get("method", "")
                    evidence_parts.append(f"{method} {uri}")

                findings.append(Finding(
                    title=alert.get("name", "Unknown Alert"),
                    severity=severity,
                    scanner="zap",
                    description=alert.get("desc", "")[:500],
                    evidence="\n".join(evidence_parts),
                    remediation=alert.get("solution", "")[:500],
                    reference=alert.get("reference", "")[:300],
                    metadata={
                        "alert_ref": alert.get("alertRef", ""),
                        "cweid": alert.get("cweid", ""),
                        "wascid": alert.get("wascid", ""),
                        "confidence": alert.get("confidence", ""),
                        "count": alert.get("count", 0),
                        "plugin_id": alert.get("pluginid", ""),
                    },
                ))

        return findings
