"""Configuration management for labtest CLI."""

from __future__ import annotations
from pathlib import Path
from typing import Optional
from urllib.parse import quote_plus

import yaml
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class LabtestConfig(BaseSettings):
    """Configuration for labtest CLI tool.

    Configuration is loaded in order of priority:
    1. CLI flags (passed as constructor args)
    2. Config file (labtest.yaml)
    3. Environment variables (LABTEST_*)
    4. Default values
    """

    model_config = SettingsConfigDict(
        env_prefix="LABTEST_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # API Configuration
    api_base_url: str = Field(
        default="",
        description="Base URL for the Kootenai API",
    )
    api_timeout: int = Field(
        default=30,
        description="API request timeout in seconds",
    )

    # Database Configuration
    db_host: str = Field(default="", description="PostgreSQL host")
    db_port: int = Field(default=5432, description="PostgreSQL port")
    db_name: str = Field(default="labctl", description="Database name")
    db_user: str = Field(default="labctl", description="Database user")
    db_password: str = Field(default="", description="Database password (required, set via LABTEST_DB_PASSWORD)")

    # NATS Configuration
    nats_url: str = Field(default="", description="NATS server URL")

    # Proxmox Configuration
    proxmox_host: str = Field(default="", description="Proxmox host")
    proxmox_port: int = Field(default=8006, description="Proxmox API port")
    proxmox_token_id: str = Field(default="", description="Proxmox API token ID")
    proxmox_token: str = Field(default="", description="Proxmox API token")
    proxmox_insecure: bool = Field(default=True, description="Skip TLS verification")

    # VM SSH Configuration
    vm_ssh_user: str = Field(default="labadmin", description="SSH user for lab VMs")
    vm_ssh_password: str = Field(default="", description="SSH password for lab VMs")
    infra_host: str = Field(default="", description="Infra VM host for SSH jump")
    jump_ssh_user: str = Field(default="labadmin", description="SSH user for jump host")
    jump_ssh_password: str = Field(default="", description="SSH password for jump host")

    # Auth Configuration
    auth_email: str = Field(
        default="demo@example.com",
        description="Email for authentication testing",
    )
    auth_password: str = Field(
        default="",
        description="Password for authentication testing (empty for demo mode)",
    )
    admin_email: str = Field(
        default="admin@example.com",
        description="Admin email for admin auth testing",
    )
    admin_password: str = Field(
        default="",
        description="Admin password for auth testing",
    )

    # Test Configuration
    test_lab_template: str = Field(
        default="Linux Foundations",
        description="Lab template to use for testing",
    )
    test_user_id: str = Field(
        default="",
        description="User ID to use for testing",
    )
    cleanup_on_success: bool = Field(
        default=True,
        description="Delete test resources after successful tests",
    )

    # Test Scope Control
    run_quick_tests: bool = Field(default=True, description="Run quick connectivity tests")
    run_full_tests: bool = Field(default=True, description="Run full E2E tests")

    # Timeouts
    pod_provision_timeout: int = Field(
        default=180,
        description="Maximum wait time for pod provisioning in seconds",
    )
    checkpoint_detect_timeout: int = Field(
        default=60,
        description="Maximum wait time for checkpoint detection in seconds",
    )

    # Output
    verbose: bool = Field(default=False, description="Enable verbose output")
    json_output: bool = Field(default=False, description="Output results as JSON")

    @classmethod
    def load(
        cls,
        config_file: Optional[Path] = None,
        **overrides,
    ) -> "LabtestConfig":
        """Load configuration from file and environment, with optional overrides.

        Args:
            config_file: Path to YAML config file (default: labtest.yaml)
            **overrides: Override values from CLI flags
        """
        file_config = {}

        # Try to load config file
        if config_file is None:
            config_file = Path("labtest.yaml")

        if config_file.exists():
            with open(config_file) as f:
                file_config = yaml.safe_load(f) or {}
                # Flatten nested config
                file_config = cls._flatten_config(file_config)

        # Merge: file config < env vars < overrides
        # pydantic-settings handles env vars automatically
        merged = {**file_config, **{k: v for k, v in overrides.items() if v is not None}}

        return cls(**merged)

    @staticmethod
    def _flatten_config(config: dict) -> dict:
        """Flatten nested YAML config to flat dict."""
        flat = {}

        # Handle scope section
        if "scope" in config:
            scope = config["scope"]
            if "quick" in scope:
                flat["run_quick_tests"] = scope["quick"]
            if "full" in scope:
                flat["run_full_tests"] = scope["full"]
            if "cleanup" in scope:
                flat["cleanup_on_success"] = scope["cleanup"]

        # Handle timeouts section
        if "timeouts" in config:
            timeouts = config["timeouts"]
            if "api" in timeouts:
                flat["api_timeout"] = timeouts["api"]
            if "pod_provision" in timeouts:
                flat["pod_provision_timeout"] = timeouts["pod_provision"]
            if "checkpoint_detect" in timeouts:
                flat["checkpoint_detect_timeout"] = timeouts["checkpoint_detect"]

        # Handle connection section
        if "connection" in config:
            conn = config["connection"]
            if "api_url" in conn:
                flat["api_base_url"] = conn["api_url"]
            if "db_host" in conn:
                flat["db_host"] = conn["db_host"]
            if "nats_url" in conn:
                flat["nats_url"] = conn["nats_url"]

        # Handle flat keys
        for key in [
            "api_base_url",
            "db_host",
            "db_port",
            "db_name",
            "db_user",
            "db_password",
            "nats_url",
            "proxmox_host",
            "test_lab_template",
            "test_user_id",
            "verbose",
        ]:
            if key in config:
                flat[key] = config[key]

        return flat

    @property
    def db_dsn(self) -> str:
        """PostgreSQL connection string."""
        return f"postgresql://{quote_plus(self.db_user)}:{quote_plus(self.db_password)}@{self.db_host}:{self.db_port}/{self.db_name}"

    @property
    def api_url(self) -> str:
        """API base URL without trailing slash."""
        return self.api_base_url.rstrip("/")

    @property
    def ws_url(self) -> str:
        """WebSocket URL derived from API URL."""
        return self.api_url.replace("http://", "ws://").replace("https://", "wss://")
