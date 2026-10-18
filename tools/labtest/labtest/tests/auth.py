"""Authentication tests for Kootenai API."""

from __future__ import annotations

import time

from httpx import HTTPStatusError

from .base import BaseTestSuite
from ..models.results import TestResult, TestSuiteResult


class AuthTestSuite(BaseTestSuite):
    """Authentication and authorization tests.

    Tests login, password change, token validation, and admin operations.
    """

    name = "Authentication"

    async def run(self) -> TestSuiteResult:
        """Run all auth tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        # Basic login tests
        results.append(
            await self.run_test(
                "Login with Demo Credentials",
                self.test_demo_login,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "Get Current User After Login",
                self.test_get_current_user,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "Login with Invalid Email",
                self.test_login_invalid_email,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "Login with Invalid Password",
                self.test_login_invalid_password,
                skip_if=lambda: not self.require_api() or not self._has_password_auth(),
                skip_reason="API client not configured or no password auth",
            )
        )

        # Token validation tests
        results.append(
            await self.run_test(
                "Authenticated Request with Valid Token",
                self.test_authenticated_request,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "Request with Invalid Token",
                self.test_invalid_token,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        results.append(
            await self.run_test(
                "Request without Token",
                self.test_unauthenticated_request,
                skip_if=lambda: not self.require_api(),
                skip_reason="API client not configured",
            )
        )

        # Password tests (only if password auth is configured)
        results.append(
            await self.run_test(
                "Password Change - Wrong Current Password",
                self.test_password_change_wrong_current,
                skip_if=lambda: not self.require_api() or not self._has_password_auth(),
                skip_reason="API client not configured or no password auth",
            )
        )

        results.append(
            await self.run_test(
                "Password Change - Weak New Password",
                self.test_password_change_weak_password,
                skip_if=lambda: not self.require_api() or not self._has_password_auth(),
                skip_reason="API client not configured or no password auth",
            )
        )

        return self.make_result(results)

    def _has_password_auth(self) -> bool:
        """Check if password authentication is configured."""
        return bool(self.config.auth_password)

    async def test_demo_login(self):
        """Test login with demo credentials (demo mode or password)."""
        # Clear any existing token
        self.api.clear_token()

        # Login with configured credentials
        response = await self.api.login(
            self.config.auth_email,
            self.config.auth_password,
        )

        assert response.token, "Login should return a token"
        assert response.user, "Login should return user info"
        assert response.user.email == self.config.auth_email, (
            f"User email mismatch: expected {self.config.auth_email}, got {response.user.email}"
        )

    async def test_get_current_user(self):
        """Test getting current user info after login."""
        # Ensure we're logged in
        await self._ensure_logged_in()

        user = await self.api.get_current_user()

        assert user.get("id"), "User should have an ID"
        assert user.get("email") == self.config.auth_email, "Email should match"

    async def test_login_invalid_email(self):
        """Test login with non-existent email fails."""
        self.api.clear_token()

        try:
            await self.api.login("nonexistent@example.com", "SomePassword123")
            raise AssertionError("Login with invalid email should fail")
        except HTTPStatusError as e:
            assert e.response.status_code in (401, 404), (
                f"Expected 401 or 404, got {e.response.status_code}"
            )

    async def test_login_invalid_password(self):
        """Test login with wrong password fails."""
        self.api.clear_token()

        try:
            await self.api.login(self.config.auth_email, "WrongPassword123!")
            raise AssertionError("Login with wrong password should fail")
        except HTTPStatusError as e:
            assert e.response.status_code == 401, (
                f"Expected 401 Unauthorized, got {e.response.status_code}"
            )

    async def test_authenticated_request(self):
        """Test that authenticated requests work with valid token."""
        await self._ensure_logged_in()

        # Make an authenticated request
        labs = await self.api.list_labs()
        # Should succeed without raising
        assert isinstance(labs, list), "Should return a list of labs"

    async def test_invalid_token(self):
        """Test that requests with invalid token are rejected."""
        # Set a fake token
        self.api.set_token("invalid.jwt.token")

        try:
            await self.api.list_labs()
            raise AssertionError("Request with invalid token should fail")
        except HTTPStatusError as e:
            assert e.response.status_code == 401, (
                f"Expected 401 Unauthorized, got {e.response.status_code}"
            )
        finally:
            self.api.clear_token()

    async def test_unauthenticated_request(self):
        """Test that protected endpoints require authentication."""
        self.api.clear_token()

        try:
            # Try to access a protected endpoint without token
            await self.api.get_current_user()
            raise AssertionError("Unauthenticated request should fail")
        except HTTPStatusError as e:
            assert e.response.status_code == 401, (
                f"Expected 401 Unauthorized, got {e.response.status_code}"
            )

    async def test_password_change_wrong_current(self):
        """Test password change with wrong current password fails."""
        await self._ensure_logged_in()

        try:
            await self.api.change_password("WrongCurrent123!", "NewPassword123!")
            raise AssertionError("Password change with wrong current should fail")
        except HTTPStatusError as e:
            assert e.response.status_code in (400, 401), (
                f"Expected 400 or 401, got {e.response.status_code}"
            )

    async def test_password_change_weak_password(self):
        """Test password change with weak new password fails."""
        await self._ensure_logged_in()

        try:
            # Try to change to a weak password (no uppercase)
            await self.api.change_password(self.config.auth_password, "weakpass1")
            raise AssertionError("Password change to weak password should fail")
        except HTTPStatusError as e:
            assert e.response.status_code == 400, (
                f"Expected 400 Bad Request, got {e.response.status_code}"
            )

    async def _ensure_logged_in(self):
        """Ensure the API client is logged in."""
        if not self.api._token:
            await self.api.login(self.config.auth_email, self.config.auth_password)


class AdminAuthTestSuite(BaseTestSuite):
    """Admin authentication and user management tests.

    Tests admin-only operations like password reset.
    Requires admin credentials in config.
    """

    name = "Admin Authentication"

    async def run(self) -> TestSuiteResult:
        """Run admin auth tests."""
        self._start_time = time.time()
        results: list[TestResult] = []

        results.append(
            await self.run_test(
                "Admin Login",
                self.test_admin_login,
                skip_if=lambda: not self._has_admin_creds(),
                skip_reason="Admin credentials not configured",
            )
        )

        results.append(
            await self.run_test(
                "Admin Password Reset - Generate",
                self.test_admin_reset_generate,
                skip_if=lambda: not self._has_admin_creds() or not self._has_test_user(),
                skip_reason="Admin credentials or test user not configured",
            )
        )

        results.append(
            await self.run_test(
                "Non-Admin Cannot Reset Passwords",
                self.test_non_admin_reset_forbidden,
                skip_if=lambda: not self.require_api() or not self._has_test_user(),
                skip_reason="API client or test user not configured",
            )
        )

        return self.make_result(results)

    def _has_admin_creds(self) -> bool:
        """Check if admin credentials are configured."""
        return bool(self.config.admin_email and self.config.admin_password)

    def _has_test_user(self) -> bool:
        """Check if a test user ID is configured."""
        return bool(self.config.test_user_id)

    async def test_admin_login(self):
        """Test admin login."""
        self.api.clear_token()

        response = await self.api.login(
            self.config.admin_email,
            self.config.admin_password,
        )

        assert response.token, "Admin login should return a token"
        assert response.user, "Admin login should return user info"
        assert "admin" in response.user.roles, (
            f"Admin user should have admin role, got {response.user.roles}"
        )

    async def test_admin_reset_generate(self):
        """Test admin can generate password for another user."""
        # Login as admin
        await self._ensure_admin_logged_in()

        # Reset password for test user with generated password
        result = await self.api.admin_reset_password(
            self.config.test_user_id,
            generate=True,
        )

        assert result.get("temporaryPassword"), (
            "Password reset should return temporary password"
        )
        assert result.get("mustChangePassword") is True, (
            "User should be required to change password"
        )

    async def test_non_admin_reset_forbidden(self):
        """Test non-admin cannot reset passwords."""
        # Login as regular user
        self.api.clear_token()
        await self.api.login(self.config.auth_email, self.config.auth_password)

        try:
            await self.api.admin_reset_password(
                self.config.test_user_id,
                generate=True,
            )
            raise AssertionError("Non-admin should not be able to reset passwords")
        except HTTPStatusError as e:
            assert e.response.status_code == 403, (
                f"Expected 403 Forbidden, got {e.response.status_code}"
            )

    async def _ensure_admin_logged_in(self):
        """Ensure logged in as admin."""
        self.api.clear_token()
        await self.api.login(self.config.admin_email, self.config.admin_password)
