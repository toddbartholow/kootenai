"""Nmap port scanner via Docker."""

from __future__ import annotations

import defusedxml.ElementTree as ET
from pathlib import Path
from tempfile import TemporaryDirectory

from ..config import SectestConfig
from ..docker_runner import DockerRunner
from ..models import Finding, ScanResult, ScanStatus, Severity
from .base import BaseScanner


class NmapScanner(BaseScanner):
    name = "nmap"
    docker_image = "instrumentisto/nmap:latest"

    def __init__(self, config: SectestConfig, runner: DockerRunner):
        super().__init__(config, runner)

    def scan(self) -> ScanResult:
        """Run nmap service/version scan against the target."""
        with TemporaryDirectory() as tmpdir:
            output_file = "/output/scan.xml"

            cmd = [
                "-sV",               # Service version detection
                "-sC",               # Default scripts
                "--top-ports", "1000",
                "-T4",               # Aggressive timing
                "-oX", output_file,  # XML output
                "--open",            # Only show open ports
                self.config.target_host,
            ]

            if self.config.nmap_extra_args:
                cmd = self.config.nmap_extra_args.split() + cmd

            exit_code, stdout, stderr = self.runner.run(
                self.docker_image,
                cmd,
                volumes={tmpdir: "/output"},
                timeout=self.config.scan_timeout,
            )

            findings: list[Finding] = []
            raw = stdout + stderr

            # Parse XML output
            xml_path = Path(tmpdir) / "scan.xml"
            if xml_path.exists():
                findings = self._parse_xml(xml_path)
                raw = xml_path.read_text()

            # Save raw output
            out = self.config.output_path / "nmap-results.xml"
            if xml_path.exists():
                out.write_text(xml_path.read_text())

            status = ScanStatus.FINDINGS if findings else ScanStatus.PASSED
            if exit_code not in (0, -1) and not findings:
                status = ScanStatus.ERROR

            return ScanResult(
                scanner=self.name,
                status=status,
                findings=findings,
                raw_output=raw[:5000],
                error=stderr if exit_code != 0 else "",
            )

    def _parse_xml(self, xml_path: Path) -> list[Finding]:
        """Parse nmap XML output into findings."""
        findings: list[Finding] = []
        try:
            tree = ET.parse(xml_path)
            root = tree.getroot()
        except ET.ParseError:
            return findings

        for host in root.findall(".//host"):
            addr_elem = host.find("address")
            addr = addr_elem.get("addr", "unknown") if addr_elem is not None else "unknown"

            for port in host.findall(".//port"):
                state_elem = port.find("state")
                if state_elem is None or state_elem.get("state") != "open":
                    continue

                portid = port.get("portid", "?")
                protocol = port.get("protocol", "tcp")
                service_elem = port.find("service")
                service_name = ""
                service_version = ""
                if service_elem is not None:
                    service_name = service_elem.get("name", "")
                    product = service_elem.get("product", "")
                    version = service_elem.get("version", "")
                    service_version = f"{product} {version}".strip()

                # Determine severity based on service
                severity = self._classify_port(int(portid), service_name)

                findings.append(Finding(
                    title=f"Open port {portid}/{protocol}: {service_name}",
                    severity=severity,
                    scanner=self.name,
                    description=f"Port {portid}/{protocol} is open running {service_version or service_name}",
                    evidence=f"{addr}:{portid} ({protocol}) - {service_version}",
                    metadata={
                        "port": portid,
                        "protocol": protocol,
                        "service": service_name,
                        "version": service_version,
                        "host": addr,
                    },
                ))

                # Check for script results (vuln scripts)
                for script in port.findall("script"):
                    script_id = script.get("id", "")
                    script_output = script.get("output", "")
                    if any(w in script_id.lower() for w in ("vuln", "exploit", "brute")):
                        findings.append(Finding(
                            title=f"NSE: {script_id} on port {portid}",
                            severity=Severity.HIGH,
                            scanner=self.name,
                            description=script_output[:500],
                            evidence=script_output[:1000],
                            metadata={"script": script_id, "port": portid},
                        ))

        return findings

    @staticmethod
    def _classify_port(port: int, service: str) -> Severity:
        """Classify the severity of an open port."""
        # Dangerous services
        dangerous = {
            21: "ftp", 23: "telnet", 445: "smb", 3389: "rdp",
            6379: "redis", 27017: "mongodb", 9200: "elasticsearch",
            11211: "memcached", 2375: "docker-unencrypted",
        }
        if port in dangerous:
            return Severity.HIGH

        # Database ports exposed
        db_ports = {3306, 5432, 1433, 1521, 27017, 6379, 9042}
        if port in db_ports:
            return Severity.HIGH

        # Admin interfaces
        admin_ports = {8443, 9090, 10000}
        if port in admin_ports:
            return Severity.MEDIUM

        # Common web services
        web_ports = {80, 443, 8080, 8443, 3000}
        if port in web_ports:
            return Severity.INFO

        # SSH
        if port == 22:
            return Severity.INFO

        # Everything else unexpected
        if port > 10000:
            return Severity.LOW

        return Severity.INFO
