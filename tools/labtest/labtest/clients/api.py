"""Async HTTP client for Kootenai API."""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import Any, Optional

import httpx

from ..exceptions import PodError, TimeoutError
from ..models.api import (
    CreatePodResponse,
    CreateSessionResponse,
    HealthResponse,
    LabTemplateListResponse,
    LoginResponse,
    Pod,
    PodListResponse,
    ReadyResponse,
    Session,
    SessionListResponse,
    SessionProgress,
    SubmitResponse,
    VersionResponse,
)

logger = logging.getLogger(__name__)


class APIClient:
    """Async HTTP client for Kootenai API.

    Usage:
        async with APIClient("http://localhost:8080").session() as client:
            health = await client.health()
    """

    def __init__(self, base_url: str, timeout: int = 30, verbose: bool = False):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.verbose = verbose
        self._client: Optional[httpx.AsyncClient] = None
        self._token: Optional[str] = None

    @asynccontextmanager
    async def session(self):
        """Context manager for HTTP client session."""
        self._client = httpx.AsyncClient(
            base_url=self.base_url,
            timeout=self.timeout,
            headers=self._headers(),
        )
        try:
            yield self
        finally:
            await self._client.aclose()
            self._client = None

    def _headers(self) -> dict[str, str]:
        """Get default headers."""
        headers = {"Content-Type": "application/json"}
        if self._token:
            headers["Authorization"] = f"Bearer {self._token}"
        return headers

    def _log_request(self, method: str, url: str, **kwargs):
        """Log request details in verbose mode."""
        if self.verbose:
            logger.info(f"Request: {method} {url}")
            if kwargs.get("json"):
                logger.debug(f"Body: {kwargs['json']}")

    def _log_response(self, resp: httpx.Response):
        """Log response details in verbose mode."""
        if self.verbose:
            logger.info(f"Response: {resp.status_code} ({resp.elapsed.total_seconds():.3f}s)")

    # Auth endpoints

    async def login(self, email: str, password: str) -> LoginResponse:
        """Login with email and password."""
        payload = {"email": email, "password": password}
        self._log_request("POST", "/api/v1/auth/login", json=payload)
        resp = await self._client.post("/api/v1/auth/login", json=payload)
        self._log_response(resp)
        resp.raise_for_status()
        data = resp.json()
        self._token = data.get("token")
        # Update client headers with new token
        if self._client:
            self._client.headers["Authorization"] = f"Bearer {self._token}"
        return LoginResponse(**data)

    async def change_password(
        self, current_password: str, new_password: str
    ) -> dict[str, Any]:
        """Change the current user's password."""
        payload = {"currentPassword": current_password, "newPassword": new_password}
        self._log_request("PUT", "/api/v1/password/change", json=payload)
        resp = await self._client.put("/api/v1/password/change", json=payload)
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    async def admin_reset_password(
        self, user_id: str, generate: bool = True, password: Optional[str] = None
    ) -> dict[str, Any]:
        """Admin reset a user's password."""
        payload: dict[str, Any] = {}
        if generate:
            payload["generatePassword"] = True
        elif password:
            payload["password"] = password
        self._log_request("PUT", f"/api/v1/users/{user_id}/password/reset", json=payload)
        resp = await self._client.put(f"/api/v1/users/{user_id}/password/reset", json=payload)
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    async def get_current_user(self) -> dict[str, Any]:
        """Get the current authenticated user."""
        self._log_request("GET", "/api/v1/auth/me")
        resp = await self._client.get("/api/v1/auth/me")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    def set_token(self, token: str):
        """Set authentication token."""
        self._token = token
        if self._client:
            self._client.headers["Authorization"] = f"Bearer {token}"

    def clear_token(self):
        """Clear authentication token."""
        self._token = None
        if self._client and "Authorization" in self._client.headers:
            del self._client.headers["Authorization"]

    # Health endpoints

    async def health(self) -> HealthResponse:
        """Check API health."""
        self._log_request("GET", "/health")
        resp = await self._client.get("/health")
        self._log_response(resp)
        resp.raise_for_status()
        return HealthResponse(**resp.json())

    async def ready(self) -> ReadyResponse:
        """Check API readiness with component health."""
        self._log_request("GET", "/ready")
        resp = await self._client.get("/ready")
        self._log_response(resp)
        # ready() may return 503 with partial health, so don't raise
        return ReadyResponse(**resp.json())

    async def version(self) -> VersionResponse:
        """Get API version information."""
        self._log_request("GET", "/version")
        resp = await self._client.get("/version")
        self._log_response(resp)
        resp.raise_for_status()
        return VersionResponse(**resp.json())

    # Lab endpoints

    async def list_labs(self) -> list[dict[str, Any]]:
        """List available lab templates."""
        self._log_request("GET", "/api/v1/labs")
        resp = await self._client.get("/api/v1/labs")
        self._log_response(resp)
        resp.raise_for_status()
        data = resp.json()
        return data.get("labs", [])

    async def get_lab(self, lab_id: str) -> dict[str, Any]:
        """Get lab template by ID."""
        self._log_request("GET", f"/api/v1/labs/{lab_id}")
        resp = await self._client.get(f"/api/v1/labs/{lab_id}")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    # Pod endpoints

    async def list_pods(self, owner: Optional[str] = None) -> PodListResponse:
        """List pods, optionally filtered by owner."""
        params = {"owner": owner} if owner else {}
        self._log_request("GET", "/api/v1/pods", params=params)
        resp = await self._client.get("/api/v1/pods", params=params)
        self._log_response(resp)
        resp.raise_for_status()
        return PodListResponse(**resp.json())

    async def create_pod(self, lab_template: str, owner_id: str) -> CreatePodResponse:
        """Create a new pod from a lab template."""
        payload = {"labTemplate": lab_template, "owner": owner_id}
        self._log_request("POST", "/api/v1/pods", json=payload)
        resp = await self._client.post("/api/v1/pods", json=payload)
        self._log_response(resp)
        resp.raise_for_status()
        return CreatePodResponse(**resp.json())

    async def get_pod(self, pod_id: str) -> Pod:
        """Get pod by ID."""
        self._log_request("GET", f"/api/v1/pods/{pod_id}")
        resp = await self._client.get(f"/api/v1/pods/{pod_id}")
        self._log_response(resp)
        resp.raise_for_status()
        return Pod(**resp.json())

    async def delete_pod(self, pod_id: str) -> dict[str, Any]:
        """Delete a pod."""
        self._log_request("DELETE", f"/api/v1/pods/{pod_id}")
        resp = await self._client.delete(f"/api/v1/pods/{pod_id}")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    async def start_pod(self, pod_id: str) -> dict[str, Any]:
        """Start a stopped pod."""
        self._log_request("POST", f"/api/v1/pods/{pod_id}/start")
        resp = await self._client.post(f"/api/v1/pods/{pod_id}/start")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    async def stop_pod(self, pod_id: str) -> dict[str, Any]:
        """Stop a running pod."""
        self._log_request("POST", f"/api/v1/pods/{pod_id}/stop")
        resp = await self._client.post(f"/api/v1/pods/{pod_id}/stop")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    async def reset_pod(self, pod_id: str) -> dict[str, Any]:
        """Reset pod to initial state."""
        self._log_request("POST", f"/api/v1/pods/{pod_id}/reset")
        resp = await self._client.post(f"/api/v1/pods/{pod_id}/reset")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    # Session endpoints

    async def list_sessions(self) -> SessionListResponse:
        """List all sessions."""
        self._log_request("GET", "/api/v1/sessions")
        resp = await self._client.get("/api/v1/sessions")
        self._log_response(resp)
        resp.raise_for_status()
        return SessionListResponse(**resp.json())

    async def create_session(
        self,
        pod_id: str,
        user_id: str,
        lab_template: str,
        lab_template_id: Optional[str] = None,
    ) -> CreateSessionResponse:
        """Create a new session for a pod."""
        payload = {
            "podId": pod_id,
            "userId": user_id,
            "labTemplate": lab_template,
        }
        if lab_template_id:
            payload["labTemplateId"] = lab_template_id
        self._log_request("POST", "/api/v1/sessions", json=payload)
        resp = await self._client.post("/api/v1/sessions", json=payload)
        self._log_response(resp)
        resp.raise_for_status()
        return CreateSessionResponse(**resp.json())

    async def get_session(self, session_id: str) -> Session:
        """Get session by ID."""
        self._log_request("GET", f"/api/v1/sessions/{session_id}")
        resp = await self._client.get(f"/api/v1/sessions/{session_id}")
        self._log_response(resp)
        resp.raise_for_status()
        return Session(**resp.json())

    async def get_progress(self, session_id: str) -> SessionProgress:
        """Get session progress with checkpoint details."""
        self._log_request("GET", f"/api/v1/sessions/{session_id}/progress")
        resp = await self._client.get(f"/api/v1/sessions/{session_id}/progress")
        self._log_response(resp)
        resp.raise_for_status()
        return SessionProgress(**resp.json())

    async def submit_session(self, session_id: str) -> SubmitResponse:
        """Submit session for grading."""
        self._log_request("POST", f"/api/v1/sessions/{session_id}/submit")
        resp = await self._client.post(f"/api/v1/sessions/{session_id}/submit")
        self._log_response(resp)
        resp.raise_for_status()
        return SubmitResponse(**resp.json())

    async def end_session(self, session_id: str) -> dict[str, Any]:
        """End a session."""
        self._log_request("POST", f"/api/v1/sessions/{session_id}/end")
        resp = await self._client.post(f"/api/v1/sessions/{session_id}/end")
        self._log_response(resp)
        resp.raise_for_status()
        return resp.json()

    # Utility methods

    async def wait_for_pod_ready(
        self,
        pod_id: str,
        timeout: int = 180,
        poll_interval: int = 5,
    ) -> Pod:
        """Wait for pod to reach running state.

        Args:
            pod_id: Pod ID to wait for
            timeout: Maximum wait time in seconds
            poll_interval: Time between status checks

        Returns:
            Pod object when ready

        Raises:
            TimeoutError: If pod doesn't become ready within timeout
            RuntimeError: If pod enters error state
        """
        import asyncio

        elapsed = 0
        while elapsed < timeout:
            pod = await self.get_pod(pod_id)

            if pod.status == "running":
                return pod
            elif pod.status == "error":
                raise PodError(f"Pod entered error state: {pod}")

            await asyncio.sleep(poll_interval)
            elapsed += poll_interval

        raise TimeoutError(f"Pod {pod_id} did not become ready within {timeout}s")
