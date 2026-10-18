"""Map trigger types to SSH commands that satisfy them."""

from __future__ import annotations

import logging
import os
import re
import shlex

from .parser import Trigger

logger = logging.getLogger(__name__)


def generate_actions(trigger: Trigger, hint: str | None = None) -> list[str]:
    """Generate SSH commands that satisfy a trigger.

    Args:
        trigger: The trigger to satisfy.
        hint: Optional hint text from the objective (may contain backtick commands).

    Returns:
        List of SSH commands to execute.
    """
    handler = _HANDLERS.get(trigger.type)
    if handler is None:
        logger.warning("No handler for trigger type: %s", trigger.type)
        return []
    return handler(trigger, hint)


# ---------------------------------------------------------------------------
# Trigger handlers
# ---------------------------------------------------------------------------

def _file_exists(trigger: Trigger, hint: str | None) -> list[str]:
    path = trigger.match.get("path", "")
    if not path:
        return []

    safe_path = shlex.quote(path)
    _, ext = os.path.splitext(path)
    if ext:
        # File with extension: ensure parent dir exists, then touch
        parent = os.path.dirname(path)
        return [f"mkdir -p {shlex.quote(parent)}", f"touch {safe_path}"]
    else:
        # No extension: treat as directory
        return [f"mkdir -p {safe_path}"]


def _file_content(trigger: Trigger, hint: str | None) -> list[str]:
    path = trigger.match.get("path", "")
    contains = trigger.match.get("contains", "")
    regex = trigger.match.get("regex", "")

    if not path:
        return []

    safe_path = shlex.quote(path)
    parent = os.path.dirname(path)
    cmds = [f"mkdir -p {shlex.quote(parent)}"]

    if contains:
        cmds.append(f"echo {shlex.quote(contains)} > {safe_path}")
    elif regex:
        # Try to produce a string matching the regex
        content = _content_from_regex(regex)
        if content:
            cmds.append(f"echo {shlex.quote(content)} > {safe_path}")
        else:
            logger.warning("Cannot auto-generate content for regex: %s", regex)
    return cmds


def _command_executed(trigger: Trigger, hint: str | None) -> list[str]:
    pattern = trigger.match.get("pattern", "")
    if not pattern:
        return []

    # Check if the pattern is mostly literal (no complex regex)
    if _is_literal(pattern):
        return [pattern]

    # Try simple wildcard expansion
    simple = _simplify_pattern(pattern)
    if simple:
        return [simple]

    # Fallback: extract backtick-quoted commands from hint
    if hint:
        cmds = _extract_hint_commands(hint)
        if cmds:
            return cmds

    logger.warning(
        "Cannot auto-generate command for pattern: %s (no hint fallback)", pattern
    )
    return []


def _package(trigger: Trigger, hint: str | None) -> list[str]:
    name = trigger.match.get("name", "")
    if not name:
        return []
    return [f"sudo apt-get install -y {shlex.quote(name)}"]


def _service(trigger: Trigger, hint: str | None) -> list[str]:
    name = trigger.match.get("name", "")
    state = trigger.match.get("state", "running")
    if not name:
        return []

    action_map = {
        "running": "start",
        "enabled": "enable",
        "stopped": "stop",
        "disabled": "disable",
    }
    action = action_map.get(state, "start")
    return [f"sudo systemctl {action} {shlex.quote(name)}"]


def _user_created(trigger: Trigger, hint: str | None) -> list[str]:
    username = trigger.match.get("username", "")
    if not username:
        return []
    return [f"sudo useradd -m {shlex.quote(username)}"]


def _permission_changed(trigger: Trigger, hint: str | None) -> list[str]:
    path = trigger.match.get("path", "")
    if not path:
        return []

    safe_path = shlex.quote(path)
    cmds: list[str] = []
    mode = trigger.match.get("mode", "")
    if mode:
        cmds.append(f"chmod {shlex.quote(mode)} {safe_path}")

    owner = trigger.match.get("owner", "")
    group = trigger.match.get("group", "")
    if owner or group:
        ownership = f"{owner}:{group}" if group else owner
        cmds.append(f"chown {shlex.quote(ownership)} {safe_path}")

    return cmds


def _network_connection(trigger: Trigger, hint: str | None) -> list[str]:
    addr = trigger.match.get("remote_addr", trigger.match.get("address", ""))
    port = trigger.match.get("port", "")
    if addr and port:
        return [f"nc -z {shlex.quote(str(addr))} {shlex.quote(str(port))}"]
    return []


def _port_listening(trigger: Trigger, hint: str | None) -> list[str]:
    port = trigger.match.get("port", "")
    if not port:
        return []
    # Use a background netcat listener; nohup so it persists briefly
    return [f"nohup nc -l -p {shlex.quote(str(port))} </dev/null >/dev/null 2>&1 &"]


def _firewall_rule(trigger: Trigger, hint: str | None) -> list[str]:
    port = trigger.match.get("port", "")
    protocol = trigger.match.get("protocol", "tcp")
    if not port:
        return []
    return [f"sudo ufw allow {shlex.quote(f'{port}/{protocol}')}"]


def _cron_job(trigger: Trigger, hint: str | None) -> list[str]:
    pattern = trigger.match.get("pattern", trigger.match.get("entry", ""))
    if not pattern:
        return []
    return [f"(crontab -l 2>/dev/null; echo {shlex.quote(pattern)}) | crontab -"]


def _active_check(trigger: Trigger, hint: str | None) -> list[str]:
    # Active checks require scenario overrides
    logger.info("Skipping active_check trigger (requires scenario override)")
    return []


# ---------------------------------------------------------------------------
# Handler registry
# ---------------------------------------------------------------------------

_HANDLERS = {
    "file_exists": _file_exists,
    "file_content": _file_content,
    "command_executed": _command_executed,
    "package": _package,
    "service": _service,
    "user_created": _user_created,
    "permission_changed": _permission_changed,
    "network_connection": _network_connection,
    "port_listening": _port_listening,
    "firewall_rule": _firewall_rule,
    "cron_job": _cron_job,
    "active_check": _active_check,
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Characters that indicate regex complexity beyond a simple literal
_REGEX_SPECIAL = set(r"^$+?{}[]|()")


def _is_literal(pattern: str) -> bool:
    """Check if a regex pattern is effectively a literal string."""
    return not any(c in _REGEX_SPECIAL for c in pattern) and ".*" not in pattern


def _simplify_pattern(pattern: str) -> str:
    """Try to simplify a regex pattern to a runnable command.

    Handles common cases like `ls.*mywork` → `ls ~/mywork`.
    """
    # Replace .* with a space (common for command + args)
    simplified = re.sub(r"\.\*", " ", pattern)
    # Strip remaining regex anchors
    simplified = simplified.strip("^$ ")
    if simplified and _is_literal(simplified):
        return simplified
    return ""


def _extract_hint_commands(hint: str) -> list[str]:
    """Extract quoted commands from a hint string (backticks or single quotes)."""
    # Match backtick-quoted or single-quoted strings
    matches = re.findall(r"`([^`]+)`|'([^']+)'", hint)
    # Flatten: each match is a tuple of (backtick_group, single_quote_group)
    candidates = [m[0] or m[1] for m in matches]
    # Filter to things that look like commands (start with a letter, slash, or tilde)
    cmds = [c for c in candidates if re.match(r"^[a-z/~]", c, re.IGNORECASE)]
    return cmds


def _content_from_regex(regex: str) -> str:
    """Attempt to produce a string that matches a simple regex.

    Only handles basic patterns. Returns empty string if too complex.
    """
    # Strip anchors
    s = regex.strip("^$")
    # If it's simple enough after removing anchors, return as-is
    if _is_literal(s):
        return s
    return ""
