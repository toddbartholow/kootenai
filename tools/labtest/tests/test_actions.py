"""Tests for simulator action generation.

Verifies that generate_actions() produces safe, properly-quoted shell
commands for each trigger type, and that special characters in paths,
usernames, and other parameters are handled via shlex.quote().
"""

from __future__ import annotations

import shlex

import pytest

from labtest.simulator.actions import (
    _content_from_regex,
    _extract_hint_commands,
    _is_literal,
    _simplify_pattern,
    generate_actions,
)
from labtest.simulator.parser import Trigger


# ---------------------------------------------------------------------------
# file_exists trigger
# ---------------------------------------------------------------------------

class TestFileExists:
    """Tests for the file_exists trigger handler."""

    def test_file_with_extension_creates_touch(self) -> None:
        trigger = Trigger(type="file_exists", target="vm", match={"path": "/home/user/report.txt"})
        cmds = generate_actions(trigger)
        assert len(cmds) == 2
        assert "mkdir -p" in cmds[0]
        assert "touch" in cmds[1]
        assert "/home/user/report.txt" in cmds[1]

    def test_directory_without_extension_creates_mkdir(self) -> None:
        trigger = Trigger(type="file_exists", target="vm", match={"path": "/opt/mydir"})
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "mkdir -p" in cmds[0]

    def test_empty_path_returns_empty(self) -> None:
        trigger = Trigger(type="file_exists", target="vm", match={"path": ""})
        assert generate_actions(trigger) == []

    def test_missing_path_returns_empty(self) -> None:
        trigger = Trigger(type="file_exists", target="vm", match={})
        assert generate_actions(trigger) == []

    def test_path_with_spaces_is_quoted(self) -> None:
        trigger = Trigger(
            type="file_exists", target="vm",
            match={"path": "/home/user/my file.txt"},
        )
        cmds = generate_actions(trigger)
        quoted = shlex.quote("/home/user/my file.txt")
        assert any(quoted in c for c in cmds)

    def test_path_with_shell_metacharacters_is_quoted(self) -> None:
        trigger = Trigger(
            type="file_exists", target="vm",
            match={"path": "/tmp/$(rm -rf /).txt"},
        )
        cmds = generate_actions(trigger)
        quoted = shlex.quote("/tmp/$(rm -rf /).txt")
        assert any(quoted in c for c in cmds)


# ---------------------------------------------------------------------------
# file_content trigger
# ---------------------------------------------------------------------------

class TestFileContent:
    """Tests for the file_content trigger handler."""

    def test_contains_writes_content(self) -> None:
        trigger = Trigger(
            type="file_content", target="vm",
            match={"path": "/tmp/hello.txt", "contains": "Hello World"},
        )
        cmds = generate_actions(trigger)
        assert any("echo" in c and "Hello World" in c for c in cmds)
        assert any("mkdir -p" in c for c in cmds)

    def test_regex_with_literal_pattern(self) -> None:
        trigger = Trigger(
            type="file_content", target="vm",
            match={"path": "/tmp/config.txt", "regex": "enabled=true"},
        )
        cmds = generate_actions(trigger)
        assert any("enabled=true" in c for c in cmds)

    def test_empty_path_returns_empty(self) -> None:
        trigger = Trigger(type="file_content", target="vm", match={"path": ""})
        assert generate_actions(trigger) == []

    def test_content_with_special_chars_is_quoted(self) -> None:
        trigger = Trigger(
            type="file_content", target="vm",
            match={"path": "/tmp/test.txt", "contains": "val=$(whoami)"},
        )
        cmds = generate_actions(trigger)
        quoted_content = shlex.quote("val=$(whoami)")
        assert any(quoted_content in c for c in cmds)


# ---------------------------------------------------------------------------
# package trigger
# ---------------------------------------------------------------------------

class TestPackage:
    """Tests for the package trigger handler."""

    def test_installs_package(self) -> None:
        trigger = Trigger(type="package", target="vm", match={"name": "nginx"})
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "apt-get install -y" in cmds[0]
        assert "nginx" in cmds[0]

    def test_empty_name_returns_empty(self) -> None:
        trigger = Trigger(type="package", target="vm", match={"name": ""})
        assert generate_actions(trigger) == []

    def test_package_name_with_injection_is_quoted(self) -> None:
        trigger = Trigger(
            type="package", target="vm",
            match={"name": "nginx; rm -rf /"},
        )
        cmds = generate_actions(trigger)
        quoted = shlex.quote("nginx; rm -rf /")
        assert quoted in cmds[0]


# ---------------------------------------------------------------------------
# service trigger
# ---------------------------------------------------------------------------

class TestService:
    """Tests for the service trigger handler."""

    @pytest.mark.parametrize(
        "state,expected_action",
        [
            ("running", "start"),
            ("enabled", "enable"),
            ("stopped", "stop"),
            ("disabled", "disable"),
        ],
    )
    def test_service_state_mapping(self, state: str, expected_action: str) -> None:
        trigger = Trigger(
            type="service", target="vm",
            match={"name": "nginx", "state": state},
        )
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert f"systemctl {expected_action}" in cmds[0]

    def test_default_state_is_running(self) -> None:
        trigger = Trigger(type="service", target="vm", match={"name": "ssh"})
        cmds = generate_actions(trigger)
        assert "start" in cmds[0]

    def test_empty_name_returns_empty(self) -> None:
        trigger = Trigger(type="service", target="vm", match={"name": ""})
        assert generate_actions(trigger) == []

    def test_service_name_with_injection_is_quoted(self) -> None:
        trigger = Trigger(
            type="service", target="vm",
            match={"name": "nginx; cat /etc/shadow"},
        )
        cmds = generate_actions(trigger)
        quoted = shlex.quote("nginx; cat /etc/shadow")
        assert quoted in cmds[0]


# ---------------------------------------------------------------------------
# user_created trigger
# ---------------------------------------------------------------------------

class TestUserCreated:
    """Tests for the user_created trigger handler."""

    def test_creates_user(self) -> None:
        trigger = Trigger(type="user_created", target="vm", match={"username": "student"})
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "useradd -m" in cmds[0]
        assert "student" in cmds[0]

    def test_empty_username_returns_empty(self) -> None:
        trigger = Trigger(type="user_created", target="vm", match={"username": ""})
        assert generate_actions(trigger) == []

    def test_username_with_injection_is_quoted(self) -> None:
        trigger = Trigger(
            type="user_created", target="vm",
            match={"username": "user; rm -rf /"},
        )
        cmds = generate_actions(trigger)
        quoted = shlex.quote("user; rm -rf /")
        assert quoted in cmds[0]


# ---------------------------------------------------------------------------
# permission_changed trigger
# ---------------------------------------------------------------------------

class TestPermissionChanged:
    """Tests for the permission_changed trigger handler."""

    def test_chmod(self) -> None:
        trigger = Trigger(
            type="permission_changed", target="vm",
            match={"path": "/tmp/file", "mode": "755"},
        )
        cmds = generate_actions(trigger)
        assert any("chmod" in c for c in cmds)

    def test_chown_owner_and_group(self) -> None:
        trigger = Trigger(
            type="permission_changed", target="vm",
            match={"path": "/tmp/file", "owner": "www-data", "group": "www-data"},
        )
        cmds = generate_actions(trigger)
        assert any("chown" in c and "www-data:www-data" in c for c in cmds)

    def test_chown_owner_only(self) -> None:
        trigger = Trigger(
            type="permission_changed", target="vm",
            match={"path": "/tmp/file", "owner": "root"},
        )
        cmds = generate_actions(trigger)
        chown_cmd = [c for c in cmds if "chown" in c]
        assert len(chown_cmd) == 1
        assert "root" in chown_cmd[0]

    def test_empty_path_returns_empty(self) -> None:
        trigger = Trigger(type="permission_changed", target="vm", match={"path": ""})
        assert generate_actions(trigger) == []


# ---------------------------------------------------------------------------
# network_connection trigger
# ---------------------------------------------------------------------------

class TestNetworkConnection:
    """Tests for the network_connection trigger handler."""

    def test_nc_command_generated(self) -> None:
        trigger = Trigger(
            type="network_connection", target="vm",
            match={"remote_addr": "10.0.0.1", "port": "22"},
        )
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "nc -z" in cmds[0]

    def test_missing_port_returns_empty(self) -> None:
        trigger = Trigger(
            type="network_connection", target="vm",
            match={"remote_addr": "10.0.0.1"},
        )
        assert generate_actions(trigger) == []


# ---------------------------------------------------------------------------
# port_listening trigger
# ---------------------------------------------------------------------------

class TestPortListening:
    """Tests for the port_listening trigger handler."""

    def test_nc_listener(self) -> None:
        trigger = Trigger(type="port_listening", target="vm", match={"port": "8080"})
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "nc -l" in cmds[0]

    def test_empty_port_returns_empty(self) -> None:
        trigger = Trigger(type="port_listening", target="vm", match={"port": ""})
        assert generate_actions(trigger) == []


# ---------------------------------------------------------------------------
# firewall_rule trigger
# ---------------------------------------------------------------------------

class TestFirewallRule:
    """Tests for the firewall_rule trigger handler."""

    def test_ufw_allow(self) -> None:
        trigger = Trigger(
            type="firewall_rule", target="vm",
            match={"port": "443", "protocol": "tcp"},
        )
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "ufw allow" in cmds[0]
        assert "443/tcp" in cmds[0]

    def test_default_protocol_tcp(self) -> None:
        trigger = Trigger(type="firewall_rule", target="vm", match={"port": "80"})
        cmds = generate_actions(trigger)
        assert "80/tcp" in cmds[0]


# ---------------------------------------------------------------------------
# cron_job trigger
# ---------------------------------------------------------------------------

class TestCronJob:
    """Tests for the cron_job trigger handler."""

    def test_crontab_entry(self) -> None:
        trigger = Trigger(
            type="cron_job", target="vm",
            match={"pattern": "0 * * * * /usr/bin/backup.sh"},
        )
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "crontab" in cmds[0]

    def test_empty_pattern_returns_empty(self) -> None:
        trigger = Trigger(type="cron_job", target="vm", match={"pattern": ""})
        assert generate_actions(trigger) == []


# ---------------------------------------------------------------------------
# command_executed trigger
# ---------------------------------------------------------------------------

class TestCommandExecuted:
    """Tests for the command_executed trigger handler."""

    def test_literal_command_returned_directly(self) -> None:
        trigger = Trigger(
            type="command_executed", target="vm",
            match={"pattern": "ls /home"},
        )
        cmds = generate_actions(trigger)
        assert cmds == ["ls /home"]

    def test_regex_pattern_simplified(self) -> None:
        trigger = Trigger(
            type="command_executed", target="vm",
            match={"pattern": "ls.*mywork"},
        )
        cmds = generate_actions(trigger)
        assert len(cmds) == 1
        assert "ls" in cmds[0]

    def test_hint_fallback_for_complex_pattern(self) -> None:
        trigger = Trigger(
            type="command_executed", target="vm",
            match={"pattern": "^(sudo\\s+)?apt-get\\s+install.*"},
        )
        hint = "Run `sudo apt-get install nginx` to install"
        cmds = generate_actions(trigger, hint=hint)
        assert "sudo apt-get install nginx" in cmds


# ---------------------------------------------------------------------------
# Unknown and active_check triggers
# ---------------------------------------------------------------------------

class TestEdgeCases:
    """Tests for unknown trigger types and active_check."""

    def test_unknown_trigger_type_returns_empty(self) -> None:
        trigger = Trigger(type="nonexistent_type", target="vm", match={})
        assert generate_actions(trigger) == []

    def test_active_check_returns_empty(self) -> None:
        trigger = Trigger(type="active_check", target="vm", match={})
        assert generate_actions(trigger) == []


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

class TestIsLiteral:
    """Tests for the _is_literal() helper."""

    @pytest.mark.parametrize(
        "pattern",
        ["ls /home", "cat file.txt", "echo hello", "systemctl start nginx"],
    )
    def test_literal_strings(self, pattern: str) -> None:
        assert _is_literal(pattern) is True

    @pytest.mark.parametrize(
        "pattern",
        ["ls.*home", "file[0-9]", "^start", "end$", "a|b", "foo(bar)", "x{2}"],
    )
    def test_regex_patterns(self, pattern: str) -> None:
        assert _is_literal(pattern) is False


class TestSimplifyPattern:
    """Tests for the _simplify_pattern() helper."""

    def test_dotstar_becomes_space(self) -> None:
        assert _simplify_pattern("ls.*mywork") == "ls mywork"

    def test_complex_pattern_returns_empty(self) -> None:
        assert _simplify_pattern("^(sudo\\s+)?apt-get") == ""

    def test_anchors_stripped(self) -> None:
        result = _simplify_pattern("^echo hello$")
        # After stripping .* (none) and anchors, should be "echo hello"
        assert result == "echo hello"


class TestExtractHintCommands:
    """Tests for the _extract_hint_commands() helper."""

    def test_backtick_commands(self) -> None:
        hint = "Run `ls -la /home` and then `cat /etc/hostname`"
        cmds = _extract_hint_commands(hint)
        assert "ls -la /home" in cmds
        assert "cat /etc/hostname" in cmds

    def test_single_quote_commands(self) -> None:
        hint = "Execute 'systemctl start nginx' to proceed"
        cmds = _extract_hint_commands(hint)
        assert "systemctl start nginx" in cmds

    def test_no_commands_found(self) -> None:
        hint = "Complete the objective by configuring the system."
        cmds = _extract_hint_commands(hint)
        assert cmds == []

    def test_non_command_strings_filtered(self) -> None:
        hint = "The file `report.txt` at `42` needs updating"
        cmds = _extract_hint_commands(hint)
        # "42" should be filtered (doesn't start with letter/slash/tilde)
        assert "42" not in cmds


class TestContentFromRegex:
    """Tests for the _content_from_regex() helper."""

    def test_literal_after_anchor_stripping(self) -> None:
        assert _content_from_regex("^hello world$") == "hello world"

    def test_complex_regex_returns_empty(self) -> None:
        assert _content_from_regex("^[a-z]+\\d{3}$") == ""

    def test_simple_string(self) -> None:
        assert _content_from_regex("enabled=true") == "enabled=true"
