"""Tests for sectest configuration.

Verifies defaults, computed properties, env var overrides, and YAML loading.
"""

from __future__ import annotations

import os
from pathlib import Path
from unittest.mock import patch

import pytest

from sectest.config import SectestConfig


class TestDefaults:
    """Verify default configuration values."""

    def test_default_target(self) -> None:
        cfg = SectestConfig()
        # No baked-in target: must be supplied via env var or config file.
        assert cfg.target_host == ""
        assert cfg.target_port == 8080
        assert cfg.target_scheme == "http"

    def test_default_docker(self) -> None:
        cfg = SectestConfig()
        assert cfg.docker_bin == "docker"
        assert cfg.docker_network == "host"

    def test_default_scan_timeout(self) -> None:
        cfg = SectestConfig()
        assert cfg.scan_timeout == 300

    def test_default_nuclei_severity(self) -> None:
        cfg = SectestConfig()
        assert cfg.nuclei_severity == "low,medium,high,critical"


class TestComputedProperties:
    """Test derived URL and path properties."""

    def test_target_url(self) -> None:
        cfg = SectestConfig(target_host="192.0.2.1", target_port=9090, target_scheme="https")
        assert cfg.target_url == "https://192.0.2.1:9090"

    def test_target_api_url(self) -> None:
        cfg = SectestConfig(target_host="192.0.2.1", target_port=8080)
        assert cfg.target_api_url == "http://192.0.2.1:8080/api/v1/"

    def test_output_path_creates_directory(self, tmp_path: Path) -> None:
        out = tmp_path / "results"
        cfg = SectestConfig(output_dir=str(out))
        path = cfg.output_path
        assert path.exists()
        assert path.is_dir()


class TestEnvOverrides:
    """Verify SECTEST_* environment variable overrides."""

    def test_env_overrides_target_host(self) -> None:
        with patch.dict(os.environ, {"SECTEST_TARGET_HOST": "198.51.100.10"}):
            cfg = SectestConfig()
            assert cfg.target_host == "198.51.100.10"

    def test_env_overrides_scan_timeout(self) -> None:
        with patch.dict(os.environ, {"SECTEST_SCAN_TIMEOUT": "600"}):
            cfg = SectestConfig()
            assert cfg.scan_timeout == 600

    def test_env_overrides_verbose(self) -> None:
        with patch.dict(os.environ, {"SECTEST_VERBOSE": "true"}):
            cfg = SectestConfig()
            assert cfg.verbose is True


class TestYAMLLoading:
    """Test loading config from YAML file."""

    def test_load_from_yaml(self, tmp_path: Path) -> None:
        config_file = tmp_path / "sectest.yaml"
        config_file.write_text(
            "target_host: 192.0.2.5\ntarget_port: 9090\nscan_timeout: 120\n"
        )
        cfg = SectestConfig.load(config_file=config_file)
        assert cfg.target_host == "192.0.2.5"
        assert cfg.target_port == 9090
        assert cfg.scan_timeout == 120

    def test_load_missing_file_uses_defaults(self) -> None:
        cfg = SectestConfig.load(config_file=Path("/nonexistent/sectest.yaml"))
        assert cfg.target_host == ""

    def test_overrides_beat_file(self, tmp_path: Path) -> None:
        config_file = tmp_path / "sectest.yaml"
        config_file.write_text("verbose: false\n")
        cfg = SectestConfig.load(config_file=config_file, verbose=True)
        assert cfg.verbose is True
