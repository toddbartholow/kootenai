"""Parse lab template YAML into structured objectives and triggers."""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional

import yaml

logger = logging.getLogger(__name__)


@dataclass
class Trigger:
    """A checkpoint trigger definition."""

    type: str
    target: str
    match: dict[str, Any] = field(default_factory=dict)


@dataclass
class Objective:
    """A graded objective from a lab template."""

    id: str
    description: str
    points: int = 0
    order: int = 0
    depends_on: list[str] = field(default_factory=list)
    triggers: list[Trigger] = field(default_factory=list)
    hint: str = ""


@dataclass
class VMSpec:
    """VM specification from a lab template."""

    name: str
    template: str
    ip: str = ""


@dataclass
class ParsedTemplate:
    """Parsed lab template with objectives and VM info."""

    name: str
    vms: list[VMSpec] = field(default_factory=list)
    objectives: list[Objective] = field(default_factory=list)
    pass_threshold: int = 70
    total_points: int = 0

    def objectives_sorted(self) -> list[Objective]:
        """Return objectives in topological order respecting depends_on."""
        completed: set[str] = set()
        sorted_objs: list[Objective] = []
        remaining = list(self.objectives)
        remaining.sort(key=lambda o: o.order)

        max_iterations = len(remaining) * len(remaining) + 1
        iterations = 0
        while remaining and iterations < max_iterations:
            iterations += 1
            for obj in list(remaining):
                if all(dep in completed for dep in obj.depends_on):
                    sorted_objs.append(obj)
                    completed.add(obj.id)
                    remaining.remove(obj)

        # Append anything left (circular deps)
        sorted_objs.extend(remaining)
        return sorted_objs


def parse_template_file(path: str | Path) -> ParsedTemplate:
    """Parse a lab template from a local YAML file.

    Args:
        path: Path to the YAML template file.

    Returns:
        ParsedTemplate with objectives and VM specs.
    """
    path = Path(path)
    if not path.exists():
        raise FileNotFoundError(f"Template not found: {path}")

    with open(path) as f:
        data = yaml.safe_load(f)

    return _parse_template_data(data)


def parse_template_api(data: dict[str, Any]) -> ParsedTemplate:
    """Parse a lab template from API response data.

    Args:
        data: Lab template dict from API.

    Returns:
        ParsedTemplate with objectives and VM specs.
    """
    return _parse_template_data(data)


def _parse_template_data(data: dict[str, Any]) -> ParsedTemplate:
    """Parse raw template data into a ParsedTemplate."""
    metadata = data.get("metadata", {})
    spec = data.get("spec", {})

    name = metadata.get("name", "Unknown Lab")

    # Parse VMs
    vms: list[VMSpec] = []
    for vm_data in spec.get("vms", []):
        ip = ""
        networks = vm_data.get("networks", [])
        if networks:
            ip = networks[0].get("ip", "")
        vms.append(VMSpec(
            name=vm_data["name"],
            template=vm_data.get("template", ""),
            ip=ip,
        ))

    # Parse checkpoints config
    checkpoints = spec.get("checkpoints", {})
    pass_threshold = checkpoints.get("pass_threshold", 70)

    # Parse objectives
    objectives: list[Objective] = []
    total_points = 0
    for obj_data in spec.get("objectives", []):
        triggers: list[Trigger] = []
        for t in obj_data.get("triggers", []):
            triggers.append(Trigger(
                type=t.get("type", ""),
                target=t.get("target", vms[0].name if vms else ""),
                match=t.get("match", {}),
            ))

        obj = Objective(
            id=obj_data["id"],
            description=obj_data.get("description", ""),
            points=obj_data.get("points", 0),
            order=obj_data.get("order", 0),
            depends_on=obj_data.get("depends_on", []),
            triggers=triggers,
            hint=obj_data.get("hint", ""),
        )
        objectives.append(obj)
        total_points += obj.points

    return ParsedTemplate(
        name=name,
        vms=vms,
        objectives=objectives,
        pass_threshold=pass_threshold,
        total_points=total_points,
    )
