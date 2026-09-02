# kootenai-python

Python utilities for the Kootenai platform. Currently provides the
`lab_templates` package: parsing and validation for lab template YAML
definitions using Pydantic models.

## What it does

- **Parse** lab template YAML (`apiVersion: v1`, `kind: LabTemplate`)
  into typed models: metadata, platform target (Proxmox/CloudStack), network
  segments, VMs, snapshots, and graded checkpoints/objectives.
- **Validate** templates beyond schema checks: duplicate VM/segment/checkpoint
  names, checkpoint triggers referencing unknown VMs or VMs without the Wazuh
  agent enabled, and circular checkpoint dependencies.
- **Model** Wazuh event payloads (file integrity, audit, package, service,
  user data) and session/grading types (`LabSession`, `CheckpointState`,
  `GradeReport`) used for objective tracking and Canvas grade sync.

## Usage

```python
from lab_templates import parse_template_file, validate_template

template = parse_template_file("labs/linux-foundations.yaml")
issues = validate_template(open("labs/linux-foundations.yaml").read())
```

A CLI entry point validates a template and prints a summary:

```bash
python -m lab_templates.parser labs/linux-foundations.yaml
```

## Install

Requires Python 3.11+.

```bash
cd python
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
```

## Tests

```bash
python -m pytest
```

Formatting and linting follow the project standards: `black`, `ruff`, and
`mypy --strict` (configured in `pyproject.toml`).
