"""Configuration management for sectest."""

from __future__ import annotations

from pathlib import Path
from typing import Optional

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class SectestConfig(BaseSettings):
    """Security test configuration with env var / file / CLI override support."""

    model_config = SettingsConfigDict(
        env_prefix="SECTEST_",
        env_file=".env",
        extra="ignore",
    )

    # Target
    target_host: str = Field(default="", description="Target host IP")
    target_port: int = Field(default=8080, description="Target API port")
    target_scheme: str = Field(default="http", description="http or https")

    # Docker
    docker_bin: str = Field(default="docker", description="Docker binary path")
    docker_network: str = Field(default="host", description="Docker network mode")

    # Output
    output_dir: str = Field(default="./sectest-results", description="Report output directory")
    verbose: bool = Field(default=False, description="Verbose output")

    # Scan options
    scan_timeout: int = Field(default=300, description="Per-scanner timeout in seconds")
    nmap_extra_args: str = Field(default="", description="Extra nmap arguments")
    nuclei_severity: str = Field(default="low,medium,high,critical", description="Nuclei severity filter")
    nuclei_extra_args: str = Field(default="", description="Extra nuclei arguments")
    zap_ajax_spider: bool = Field(default=False, description="Enable ZAP AJAX spider")

    @property
    def target_url(self) -> str:
        return f"{self.target_scheme}://{self.target_host}:{self.target_port}"

    @property
    def target_api_url(self) -> str:
        return f"{self.target_url}/api/v1/"

    @property
    def output_path(self) -> Path:
        p = Path(self.output_dir)
        p.mkdir(parents=True, exist_ok=True)
        return p

    @classmethod
    def load(cls, config_file: Optional[Path] = None, **overrides) -> SectestConfig:
        """Load config with precedence: file < env < overrides."""
        file_values = {}
        if config_file and config_file.exists():
            import yaml

            with open(config_file) as f:
                raw = yaml.safe_load(f) or {}
            file_values = {k: v for k, v in raw.items() if v is not None}

        merged = {**file_values, **{k: v for k, v in overrides.items() if v is not None}}
        return cls(**merged)
