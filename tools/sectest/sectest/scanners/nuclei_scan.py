"""Nuclei vulnerability scanner via Docker."""

from __future__ import annotations

import json
from pathlib import Path
from tempfile import TemporaryDirectory

from ..config import SectestConfig
from ..docker_runner import DockerRunner
from ..models import Finding, ScanResult, ScanStatus, Severity
from .base import BaseScanner

# Map nuclei severity strings to our model
_SEVERITY_MAP = {
    "critical": Severity.CRITICAL,
    "high": Severity.HIGH,
    "medium": Severity.MEDIUM,
    "low": Severity.LOW,
    "info": Severity.INFO,
    "unknown": Severity.INFO,
}


class NucleiScanner(BaseScanner):
    name = "nuclei"
    docker_image = "projectdiscovery/nuclei:latest"

    def __init__(self, config: SectestConfig, runner: DockerRunner):
        super().__init__(config, runner)

    def scan(self) -> ScanResult:
        """Run nuclei against the target URL."""
        with TemporaryDirectory() as tmpdir:
            cmd = [
                "-u", self.config.target_url,
                "-jsonl",
                "-o", "/output/results.jsonl",
                "-severity", self.config.nuclei_severity,
                "-silent",
                "-timeout", "10",
                "-retries", "1",
                "-no-color",
                "-stats",
            ]

            if self.config.nuclei_extra_args:
                cmd.extend(self.config.nuclei_extra_args.split())

            exit_code, stdout, stderr = self.runner.run(
                self.docker_image,
                cmd,
                volumes={tmpdir: "/output"},
                timeout=self.config.scan_timeout,
            )

            findings: list[Finding] = []
            raw = stdout

            # Parse JSONL output
            results_path = Path(tmpdir) / "results.jsonl"
            if results_path.exists():
                raw = results_path.read_text()
                findings = self._parse_jsonl(raw)

                # Save to output dir
                out = self.config.output_path / "nuclei-results.jsonl"
                out.write_text(raw)

            status = ScanStatus.FINDINGS if findings else ScanStatus.PASSED
            if exit_code not in (0,) and not findings:
                # nuclei returns 0 even with findings; non-zero may mean error
                if "error" in stderr.lower() and not findings:
                    status = ScanStatus.ERROR

            return ScanResult(
                scanner=self.name,
                status=status,
                findings=findings,
                raw_output=raw[:5000],
                error=stderr[:2000] if exit_code != 0 and not findings else "",
            )

    @staticmethod
    def _parse_jsonl(raw: str) -> list[Finding]:
        """Parse nuclei JSONL output."""
        findings: list[Finding] = []
        for line in raw.strip().splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue

            info = entry.get("info", {})
            severity_str = info.get("severity", "info").lower()
            severity = _SEVERITY_MAP.get(severity_str, Severity.INFO)

            matched = entry.get("matched-at", entry.get("host", ""))
            template_id = entry.get("template-id", "unknown")
            name = info.get("name", template_id)

            findings.append(Finding(
                title=f"{name}",
                severity=severity,
                scanner="nuclei",
                description=info.get("description", ""),
                evidence=f"Matched: {matched}",
                remediation=info.get("remediation", ""),
                reference=", ".join(info.get("reference", [])[:3]),
                metadata={
                    "template_id": template_id,
                    "matched_at": matched,
                    "tags": info.get("tags", []),
                    "matcher_name": entry.get("matcher-name", ""),
                    "curl_command": entry.get("curl-command", ""),
                },
            ))

        return findings
