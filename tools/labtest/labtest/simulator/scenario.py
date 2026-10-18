"""Load optional scenario override YAML files."""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

import yaml

logger = logging.getLogger(__name__)


@dataclass
class ScenarioStep:
    """A single step in a scenario override."""

    objective: str
    target: str = ""
    actions: list[str] = field(default_factory=list)
    delay_after: float = 0


@dataclass
class Scenario:
    """A scenario override for a lab template."""

    lab_template: str = ""
    steps: list[ScenarioStep] = field(default_factory=list)

    def get_step(self, objective_id: str) -> Optional[ScenarioStep]:
        """Get the scenario step for an objective, if defined."""
        for step in self.steps:
            if step.objective == objective_id:
                return step
        return None


def load_scenario(path: str | Path) -> Scenario:
    """Load a scenario override from a YAML file.

    Args:
        path: Path to the scenario YAML file.

    Returns:
        Scenario with step overrides.
    """
    path = Path(path)
    if not path.exists():
        raise FileNotFoundError(f"Scenario file not found: {path}")

    with open(path) as f:
        data = yaml.safe_load(f) or {}

    steps: list[ScenarioStep] = []
    for step_data in data.get("steps", []):
        steps.append(ScenarioStep(
            objective=step_data.get("objective", ""),
            target=step_data.get("target", ""),
            actions=step_data.get("actions", []),
            delay_after=step_data.get("delay_after", 0),
        ))

    return Scenario(
        lab_template=data.get("lab_template", ""),
        steps=steps,
    )
