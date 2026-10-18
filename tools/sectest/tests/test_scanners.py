"""Tests for scanner output parsers.

Tests the static _parse_* methods on each scanner class to verify
they correctly transform raw tool output into Finding objects,
without running actual Docker containers.
"""

from __future__ import annotations

import json
import textwrap

import pytest

from sectest.models import Finding, Severity
from sectest.scanners.nuclei_scan import NucleiScanner
from sectest.scanners.ssl_scan import SslScanner
from sectest.scanners.zap_scan import ZapScanner


# ---------------------------------------------------------------------------
# Nuclei JSONL parsing
# ---------------------------------------------------------------------------

class TestNucleiParser:
    """Test NucleiScanner._parse_jsonl()."""

    def test_parse_single_finding(self) -> None:
        entry = {
            "template-id": "cve-2021-1234",
            "info": {
                "name": "Test CVE",
                "severity": "high",
                "description": "A vulnerability",
                "remediation": "Upgrade",
                "reference": ["https://example.com"],
                "tags": ["cve"],
            },
            "matched-at": "http://localhost:8080/api",
            "matcher-name": "body",
        }
        raw = json.dumps(entry) + "\n"
        findings = NucleiScanner._parse_jsonl(raw)
        assert len(findings) == 1
        assert findings[0].severity == Severity.HIGH
        assert findings[0].title == "Test CVE"
        assert findings[0].scanner == "nuclei"
        assert "cve-2021-1234" in findings[0].metadata["template_id"]

    def test_parse_multiple_findings(self) -> None:
        lines = []
        for sev in ("critical", "medium", "info"):
            lines.append(json.dumps({
                "template-id": f"tmpl-{sev}",
                "info": {"name": f"Finding {sev}", "severity": sev},
                "matched-at": "http://localhost",
            }))
        raw = "\n".join(lines) + "\n"
        findings = NucleiScanner._parse_jsonl(raw)
        assert len(findings) == 3
        severities = {f.severity for f in findings}
        assert Severity.CRITICAL in severities
        assert Severity.MEDIUM in severities
        assert Severity.INFO in severities

    def test_parse_empty_output(self) -> None:
        assert NucleiScanner._parse_jsonl("") == []

    def test_parse_invalid_json_lines_skipped(self) -> None:
        raw = "not json\n" + json.dumps({
            "template-id": "valid",
            "info": {"name": "Valid", "severity": "low"},
            "matched-at": "http://localhost",
        }) + "\n"
        findings = NucleiScanner._parse_jsonl(raw)
        assert len(findings) == 1

    def test_unknown_severity_maps_to_info(self) -> None:
        entry = {
            "template-id": "test",
            "info": {"name": "Unknown Sev", "severity": "unknown"},
            "matched-at": "http://localhost",
        }
        findings = NucleiScanner._parse_jsonl(json.dumps(entry))
        assert findings[0].severity == Severity.INFO


# ---------------------------------------------------------------------------
# ZAP JSON report parsing
# ---------------------------------------------------------------------------

class TestZapParser:
    """Test ZapScanner._parse_json_report()."""

    def test_parse_zap_report(self) -> None:
        report = {
            "site": [
                {
                    "alerts": [
                        {
                            "name": "SQL Injection",
                            "riskcode": "3",
                            "desc": "SQL injection found",
                            "solution": "Use parameterized queries",
                            "reference": "https://owasp.org",
                            "instances": [
                                {"uri": "http://localhost/api/users", "method": "GET"},
                            ],
                            "alertRef": "40018",
                            "cweid": "89",
                            "wascid": "19",
                            "confidence": "3",
                            "count": "1",
                            "pluginid": "40018",
                        },
                    ],
                },
            ],
        }
        raw = json.dumps(report)
        findings = ZapScanner._parse_json_report(raw)
        assert len(findings) == 1
        assert findings[0].severity == Severity.HIGH  # riskcode 3 = HIGH
        assert findings[0].title == "SQL Injection"
        assert findings[0].scanner == "zap"

    def test_parse_empty_report(self) -> None:
        assert ZapScanner._parse_json_report("{}") == []

    def test_parse_invalid_json(self) -> None:
        assert ZapScanner._parse_json_report("not json") == []

    def test_parse_multiple_alerts(self) -> None:
        report = {
            "site": [
                {
                    "alerts": [
                        {"name": "XSS", "riskcode": "2", "instances": []},
                        {"name": "Info Leak", "riskcode": "0", "instances": []},
                    ],
                },
            ],
        }
        findings = ZapScanner._parse_json_report(json.dumps(report))
        assert len(findings) == 2
        severities = {f.severity for f in findings}
        assert Severity.MEDIUM in severities  # riskcode 2
        assert Severity.INFO in severities  # riskcode 0


# ---------------------------------------------------------------------------
# SSL/TLS JSON parsing
# ---------------------------------------------------------------------------

class TestSslParser:
    """Test SslScanner._parse_json()."""

    def test_parse_findings(self) -> None:
        data = [
            {"id": "BEAST", "severity": "MEDIUM", "finding": "BEAST attack possible", "ip": "10.0.0.1"},
            {"id": "heartbleed", "severity": "CRITICAL", "finding": "Heartbleed vulnerable", "ip": "10.0.0.1"},
            {"id": "cert_valid", "severity": "OK", "finding": "Certificate is valid", "ip": "10.0.0.1"},
        ]
        findings = SslScanner._parse_json(json.dumps(data))
        # OK/INFO entries should be filtered out
        assert len(findings) == 2
        assert any(f.severity == Severity.CRITICAL for f in findings)
        assert any(f.severity == Severity.MEDIUM for f in findings)

    def test_parse_empty_json(self) -> None:
        assert SslScanner._parse_json("[]") == []

    def test_parse_invalid_json(self) -> None:
        assert SslScanner._parse_json("broken") == []

    def test_parse_scanresult_wrapper(self) -> None:
        """Handle the alternative format with scanResult key."""
        data = {
            "scanResult": [
                {"id": "POODLE", "severity": "HIGH", "finding": "POODLE attack"},
            ],
        }
        findings = SslScanner._parse_json(json.dumps(data))
        assert len(findings) == 1
        assert findings[0].severity == Severity.HIGH

    def test_warn_severity_maps_to_medium(self) -> None:
        data = [{"id": "weak-cipher", "severity": "WARN", "finding": "Weak cipher suite"}]
        findings = SslScanner._parse_json(json.dumps(data))
        assert len(findings) == 1
        assert findings[0].severity == Severity.MEDIUM
