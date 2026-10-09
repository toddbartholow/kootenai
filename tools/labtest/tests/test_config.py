"""Tests for labtest configuration loading.

Verifies default values, DSN construction, environment variable overrides,
YAML config file loading, and property derivations.
"""

from __future__ import annotations

import os
import textwrap
from pathlib import Path
from unittest.mock import patch

import pytest

from labtest.config import LabtestConfig


class TestDefaults:
    """Verify sensible default values."""

    def test_default_api_url(self) -> None:
        cfg = LabtestConfig()
        # No baked-in endpoint: must be supplied via env var or config file.
        assert cfg.api_base_url == ""

    def test_default_db_settings(self) -> None:
        cfg = LabtestConfig()
        assert cfg.db_host == ""
        assert cfg.db_port == 5432
        assert cfg.db_name == "labctl"
        assert cfg.db_user == "labctl"

    def test_default_nats_url(self) -> None:
        cfg = LabtestConfig()
        assert cfg.nats_url == ""

    def test_default_timeouts(self) -> None:
        cfg = LabtestConfig()
        assert cfg.api_timeout == 30
        assert cfg.pod_provision_timeout == 180
        assert cfg.checkpoint_detect_timeout == 60

    def test_default_test_scope(self) -> None:
        cfg = LabtestConfig()
        assert cfg.run_quick_tests is True
        assert cfg.run_full_tests is True
        assert cfg.cleanup_on_success is True

    def test_default_verbose_off(self) -> None:
        cfg = LabtestConfig()
        assert cfg.verbose is False
        assert cfg.json_output is False


class TestDSNProperty:
    """Verify the db_dsn property builds a correct connection string."""

    def test_dsn_format(self) -> None:
        cfg = LabtestConfig(
            db_user="myuser",
            db_password="mypass",
            db_host="dbhost",
            db_port=5433,
            db_name="mydb",
        )
        assert cfg.db_dsn == "postgresql://myuser:mypass@dbhost:5433/mydb"

    def test_dsn_with_special_chars_in_password(self) -> None:
        """Password with special characters is URL-encoded in the DSN.

        The implementation runs user and password through quote_plus so
        reserved characters (@ : /) cannot corrupt the connection string.
        """
        cfg = LabtestConfig(db_password="p@ss:w0rd/special")
        assert "p%40ss%3Aw0rd%2Fspecial" in cfg.db_dsn
        assert "p@ss:w0rd/special" not in cfg.db_dsn

    def test_dsn_with_default_values(self) -> None:
        cfg = LabtestConfig()
        dsn = cfg.db_dsn
        assert dsn.startswith("postgresql://")
        assert "labctl" in dsn  # default user and db name


class TestURLProperties:
    """Verify URL derivation properties."""

    def test_api_url_strips_trailing_slash(self) -> None:
        cfg = LabtestConfig(api_base_url="http://host:8080/")
        assert cfg.api_url == "http://host:8080"

    def test_api_url_no_trailing_slash_unchanged(self) -> None:
        cfg = LabtestConfig(api_base_url="http://host:8080")
        assert cfg.api_url == "http://host:8080"

    def test_ws_url_from_http(self) -> None:
        cfg = LabtestConfig(api_base_url="http://host:8080")
        assert cfg.ws_url == "ws://host:8080"

    def test_ws_url_from_https(self) -> None:
        cfg = LabtestConfig(api_base_url="https://host:8443")
        assert cfg.ws_url == "wss://host:8443"


class TestEnvironmentVariableOverrides:
    """Verify that LABTEST_* env vars override defaults."""

    def test_env_overrides_api_url(self) -> None:
        with patch.dict(os.environ, {"LABTEST_API_BASE_URL": "http://custom:9090"}):
            cfg = LabtestConfig()
            assert cfg.api_base_url == "http://custom:9090"

    def test_env_overrides_db_host(self) -> None:
        with patch.dict(os.environ, {"LABTEST_DB_HOST": "198.51.100.99"}):
            cfg = LabtestConfig()
            assert cfg.db_host == "198.51.100.99"

    def test_env_overrides_verbose(self) -> None:
        with patch.dict(os.environ, {"LABTEST_VERBOSE": "true"}):
            cfg = LabtestConfig()
            assert cfg.verbose is True

    def test_env_overrides_timeout(self) -> None:
        with patch.dict(os.environ, {"LABTEST_POD_PROVISION_TIMEOUT": "300"}):
            cfg = LabtestConfig()
            assert cfg.pod_provision_timeout == 300


class TestYAMLConfigLoading:
    """Verify loading from a YAML config file."""

    def test_load_from_yaml_file(self, tmp_path: Path) -> None:
        config_file = tmp_path / "labtest.yaml"
        config_file.write_text(textwrap.dedent("""\
            connection:
              api_url: http://192.0.2.1:8080
              db_host: 192.0.2.2
            scope:
              quick: true
              full: false
              cleanup: false
            timeouts:
              api: 60
              pod_provision: 300
        """))
        cfg = LabtestConfig.load(config_file=config_file)
        assert cfg.api_base_url == "http://192.0.2.1:8080"
        assert cfg.db_host == "192.0.2.2"
        assert cfg.run_quick_tests is True
        assert cfg.run_full_tests is False
        assert cfg.cleanup_on_success is False
        assert cfg.api_timeout == 60
        assert cfg.pod_provision_timeout == 300

    def test_load_with_flat_keys(self, tmp_path: Path) -> None:
        config_file = tmp_path / "labtest.yaml"
        config_file.write_text(textwrap.dedent("""\
            api_base_url: http://flat-style:8080
            db_host: flat-db
            verbose: true
        """))
        cfg = LabtestConfig.load(config_file=config_file)
        assert cfg.api_base_url == "http://flat-style:8080"
        assert cfg.db_host == "flat-db"
        assert cfg.verbose is True

    def test_load_nonexistent_file_uses_defaults(self, tmp_path: Path) -> None:
        missing = tmp_path / "nonexistent.yaml"
        cfg = LabtestConfig.load(config_file=missing)
        # Should fall back to defaults without error
        assert cfg.api_base_url == ""

    def test_load_empty_yaml_uses_defaults(self, tmp_path: Path) -> None:
        config_file = tmp_path / "labtest.yaml"
        config_file.write_text("")
        cfg = LabtestConfig.load(config_file=config_file)
        assert cfg.api_base_url == ""

    def test_overrides_take_precedence_over_file(self, tmp_path: Path) -> None:
        config_file = tmp_path / "labtest.yaml"
        config_file.write_text("verbose: false\n")
        cfg = LabtestConfig.load(config_file=config_file, verbose=True)
        assert cfg.verbose is True

    def test_none_overrides_are_ignored(self, tmp_path: Path) -> None:
        config_file = tmp_path / "labtest.yaml"
        config_file.write_text("verbose: true\n")
        cfg = LabtestConfig.load(config_file=config_file, verbose=None)
        assert cfg.verbose is True


class TestFlattenConfig:
    """Test the _flatten_config static method directly."""

    def test_flatten_scope(self) -> None:
        nested = {"scope": {"quick": False, "full": True, "cleanup": False}}
        flat = LabtestConfig._flatten_config(nested)
        assert flat["run_quick_tests"] is False
        assert flat["run_full_tests"] is True
        assert flat["cleanup_on_success"] is False

    def test_flatten_timeouts(self) -> None:
        nested = {"timeouts": {"api": 45, "pod_provision": 200, "checkpoint_detect": 90}}
        flat = LabtestConfig._flatten_config(nested)
        assert flat["api_timeout"] == 45
        assert flat["pod_provision_timeout"] == 200
        assert flat["checkpoint_detect_timeout"] == 90

    def test_flatten_connection(self) -> None:
        nested = {
            "connection": {
                "api_url": "http://example:8080",
                "db_host": "db.example",
                "nats_url": "nats://example:4222",
            }
        }
        flat = LabtestConfig._flatten_config(nested)
        assert flat["api_base_url"] == "http://example:8080"
        assert flat["db_host"] == "db.example"
        assert flat["nats_url"] == "nats://example:4222"

    def test_flatten_empty_dict(self) -> None:
        flat = LabtestConfig._flatten_config({})
        assert flat == {}
