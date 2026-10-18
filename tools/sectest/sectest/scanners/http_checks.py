"""Native HTTP security checks (no Docker required)."""

from __future__ import annotations

import time

import httpx

from ..config import SectestConfig
from ..docker_runner import DockerRunner
from ..models import Finding, ScanResult, ScanStatus, Severity
from .base import BaseScanner

# Security headers that should be present
EXPECTED_HEADERS = {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": None,  # Any value is ok
    "X-XSS-Protection": None,
    "Strict-Transport-Security": None,
    "Content-Security-Policy": None,
    "Referrer-Policy": None,
    "Permissions-Policy": None,
}

# Headers that should NOT be present (information leakage)
UNWANTED_HEADERS = {
    "Server",
    "X-Powered-By",
    "X-AspNet-Version",
    "X-AspNetMvc-Version",
}

# Common paths to check for exposure
SENSITIVE_PATHS = [
    "/.env",
    "/.git/config",
    "/.git/HEAD",
    "/debug/pprof/",
    "/debug/vars",
    "/metrics",
    "/actuator",
    "/swagger-ui/",
    "/api-docs",
    "/.well-known/openid-configuration",
    "/server-status",
    "/server-info",
    "/phpinfo.php",
    "/wp-admin/",
    "/admin/",
    "/console/",
    "/graphql",
]


class HttpSecurityChecker(BaseScanner):
    """Runs HTTP-level security checks without Docker."""

    name = "http-security"
    docker_image = ""  # No Docker needed

    def __init__(self, config: SectestConfig, runner: DockerRunner):
        super().__init__(config, runner)

    def scan(self) -> ScanResult:
        """Run all HTTP security checks."""
        findings: list[Finding] = []
        raw_parts: list[str] = []

        try:
            with httpx.Client(timeout=10, follow_redirects=True, verify=False) as client:
                # 1. Security headers check
                hdr_findings, hdr_raw = self._check_headers(client)
                findings.extend(hdr_findings)
                raw_parts.append(hdr_raw)

                # 2. Sensitive path exposure
                path_findings, path_raw = self._check_sensitive_paths(client)
                findings.extend(path_findings)
                raw_parts.append(path_raw)

                # 3. CORS misconfiguration
                cors_findings, cors_raw = self._check_cors(client)
                findings.extend(cors_findings)
                raw_parts.append(cors_raw)

                # 4. HTTP methods
                method_findings, method_raw = self._check_methods(client)
                findings.extend(method_findings)
                raw_parts.append(method_raw)

                # 5. Error handling information leakage
                err_findings, err_raw = self._check_error_leakage(client)
                findings.extend(err_findings)
                raw_parts.append(err_raw)

        except httpx.ConnectError as e:
            return ScanResult(
                scanner=self.name,
                status=ScanStatus.ERROR,
                error=f"Cannot connect to {self.config.target_url}: {e}",
            )

        status = ScanStatus.FINDINGS if findings else ScanStatus.PASSED
        return ScanResult(
            scanner=self.name,
            status=status,
            findings=findings,
            raw_output="\n---\n".join(raw_parts)[:5000],
        )

    def _check_headers(self, client: httpx.Client) -> tuple[list[Finding], str]:
        """Check for missing security headers."""
        findings: list[Finding] = []
        resp = client.get(self.config.target_url)
        raw = f"GET {self.config.target_url} -> {resp.status_code}\nHeaders: {dict(resp.headers)}"

        # Check for missing headers
        for header, expected_value in EXPECTED_HEADERS.items():
            value = resp.headers.get(header)
            if value is None:
                findings.append(Finding(
                    title=f"Missing security header: {header}",
                    severity=Severity.LOW,
                    scanner=self.name,
                    description=f"The HTTP response is missing the {header} header",
                    remediation=f"Add {header} header to responses"
                        + (f" with value '{expected_value}'" if expected_value else ""),
                    metadata={"header": header},
                ))
            elif expected_value and value.lower() != expected_value.lower():
                findings.append(Finding(
                    title=f"Incorrect {header} header value",
                    severity=Severity.LOW,
                    scanner=self.name,
                    description=f"{header} is '{value}', expected '{expected_value}'",
                    metadata={"header": header, "actual": value, "expected": expected_value},
                ))

        # Check for unwanted headers
        for header in UNWANTED_HEADERS:
            value = resp.headers.get(header)
            if value:
                findings.append(Finding(
                    title=f"Information leakage: {header} header",
                    severity=Severity.LOW,
                    scanner=self.name,
                    description=f"{header} header reveals: {value}",
                    evidence=f"{header}: {value}",
                    remediation=f"Remove the {header} header from responses",
                    metadata={"header": header, "value": value},
                ))

        return findings, raw

    def _check_sensitive_paths(self, client: httpx.Client) -> tuple[list[Finding], str]:
        """Check for exposed sensitive paths."""
        findings: list[Finding] = []
        raw_parts: list[str] = []

        for path in SENSITIVE_PATHS:
            url = f"{self.config.target_url}{path}"
            try:
                resp = client.get(url, follow_redirects=False)
                raw_parts.append(f"GET {path} -> {resp.status_code}")

                if resp.status_code == 200:
                    # Check if it's a real response (not a custom 404)
                    content = resp.text[:200].lower()
                    if "not found" not in content and "404" not in content:
                        severity = Severity.HIGH if path in ("/.env", "/.git/config", "/.git/HEAD") else Severity.MEDIUM
                        findings.append(Finding(
                            title=f"Sensitive path accessible: {path}",
                            severity=severity,
                            scanner=self.name,
                            description=f"The path {path} returned HTTP 200",
                            evidence=f"GET {url} -> 200\nBody preview: {resp.text[:200]}",
                            remediation=f"Restrict access to {path}",
                            metadata={"path": path, "status": 200},
                        ))
            except httpx.HTTPError:
                continue

        return findings, "\n".join(raw_parts)

    def _check_cors(self, client: httpx.Client) -> tuple[list[Finding], str]:
        """Check for CORS misconfigurations."""
        findings: list[Finding] = []

        # Test with a malicious origin
        evil_origin = "https://evil.example.com"
        try:
            resp = client.options(
                self.config.target_api_url,
                headers={
                    "Origin": evil_origin,
                    "Access-Control-Request-Method": "GET",
                },
            )
            raw = f"OPTIONS {self.config.target_api_url}\nOrigin: {evil_origin}\n-> {resp.status_code}\nHeaders: {dict(resp.headers)}"

            acao = resp.headers.get("Access-Control-Allow-Origin", "")
            if acao == "*":
                findings.append(Finding(
                    title="CORS: Wildcard Access-Control-Allow-Origin",
                    severity=Severity.MEDIUM,
                    scanner=self.name,
                    description="The API returns Access-Control-Allow-Origin: *, allowing any origin",
                    evidence=f"Access-Control-Allow-Origin: {acao}",
                    remediation="Restrict CORS to specific trusted origins",
                ))
            elif acao == evil_origin:
                findings.append(Finding(
                    title="CORS: Reflects arbitrary origin",
                    severity=Severity.HIGH,
                    scanner=self.name,
                    description=f"The API reflected the malicious origin {evil_origin} in ACAO header",
                    evidence=f"Origin: {evil_origin} -> ACAO: {acao}",
                    remediation="Validate CORS origin against an allowlist",
                ))

            # Check if credentials are allowed with wildcard
            acac = resp.headers.get("Access-Control-Allow-Credentials", "")
            if acac.lower() == "true" and acao in ("*", evil_origin):
                findings.append(Finding(
                    title="CORS: Credentials allowed with permissive origin",
                    severity=Severity.HIGH,
                    scanner=self.name,
                    description="Credentials are allowed with a wildcard or reflected origin",
                    evidence=f"ACAO: {acao}, ACAC: {acac}",
                    remediation="Do not allow credentials with wildcard origins",
                ))
        except httpx.HTTPError:
            raw = "CORS check failed (connection error)"

        return findings, raw

    def _check_methods(self, client: httpx.Client) -> tuple[list[Finding], str]:
        """Check for dangerous HTTP methods."""
        findings: list[Finding] = []
        dangerous = ["TRACE", "TRACK", "PUT", "DELETE"]
        raw_parts: list[str] = []

        for method in dangerous:
            try:
                resp = client.request(method, self.config.target_url)
                raw_parts.append(f"{method} {self.config.target_url} -> {resp.status_code}")

                if resp.status_code not in (404, 405, 501, 403):
                    if method in ("TRACE", "TRACK"):
                        findings.append(Finding(
                            title=f"HTTP {method} method enabled",
                            severity=Severity.MEDIUM,
                            scanner=self.name,
                            description=f"{method} method returned {resp.status_code} (should be 405)",
                            evidence=f"{method} -> {resp.status_code}",
                            remediation=f"Disable HTTP {method} method",
                        ))
            except httpx.HTTPError:
                continue

        return findings, "\n".join(raw_parts)

    def _check_error_leakage(self, client: httpx.Client) -> tuple[list[Finding], str]:
        """Check if error responses leak internal details."""
        findings: list[Finding] = []
        raw_parts: list[str] = []

        # Test with various bad inputs
        test_cases = [
            (f"{self.config.target_api_url}nonexistent-endpoint-12345", "GET", None),
            (f"{self.config.target_api_url}labs", "POST", "{{invalid json}}"),
        ]

        leak_patterns = [
            "stack trace", "traceback", "goroutine", "panic:",
            "at /", "runtime/", "net/http", "sql:",
            "file not found", "/home/", "/var/", "/usr/",
            "connection refused", "dial tcp",
        ]

        for url, method, body in test_cases:
            try:
                if body:
                    resp = client.request(method, url, content=body, headers={"Content-Type": "application/json"})
                else:
                    resp = client.request(method, url)

                raw_parts.append(f"{method} {url} -> {resp.status_code}")
                text = resp.text.lower()

                for pattern in leak_patterns:
                    if pattern in text:
                        findings.append(Finding(
                            title=f"Error response leaks internal details",
                            severity=Severity.MEDIUM,
                            scanner=self.name,
                            description=f"Error response contains '{pattern}'",
                            evidence=f"{method} {url} -> {resp.status_code}\n{resp.text[:300]}",
                            remediation="Return generic error messages without internal details",
                            metadata={"pattern": pattern, "url": url},
                        ))
                        break  # One finding per endpoint
            except httpx.HTTPError:
                continue

        return findings, "\n".join(raw_parts)
