"""Lab template parser and validator."""

from __future__ import annotations

from datetime import datetime
from enum import Enum
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, Field, ValidationError, field_validator, model_validator

# -----------------------------------------------------------------------------
# API versions
# -----------------------------------------------------------------------------

# The schema identifier every lab template carries in its `apiVersion` field.
#
# `v1` is canonical and matches the Go side, which already defaults to it in
# models.NewLabTemplate and templates/inheritance.go.
CURRENT_API_VERSION = "v1"

# Values still accepted when reading existing templates, but never emitted.
#
# `virtuallab.dev/v1` was the Python default for the project's first few months
# and is written into templates authored in that window. The domain belongs to a
# third party, not to this project, so it must not be produced for anything new.
LEGACY_API_VERSIONS = frozenset({"virtuallab.dev/v1"})

SUPPORTED_API_VERSIONS = frozenset({CURRENT_API_VERSION}) | LEGACY_API_VERSIONS


# -----------------------------------------------------------------------------
# Enums
# -----------------------------------------------------------------------------

class Platform(str, Enum):
    PROXMOX = "proxmox"
    CLOUDSTACK = "cloudstack"
    ANY = "any"


class Difficulty(str, Enum):
    BEGINNER = "beginner"
    INTERMEDIATE = "intermediate"
    ADVANCED = "advanced"
    EXPERT = "expert"


class TriggerType(str, Enum):
    """Mirrors models.AllTriggerTypes in api/internal/models/checkpoint.go."""

    FILE_EXISTS = "file_exists"
    FILE_CONTENT = "file_content"
    FILE_DELETED = "file_deleted"
    PACKAGE = "package"
    SERVICE = "service"
    COMMAND_EXECUTED = "command_executed"
    USER_CREATED = "user_created"
    PERMISSION_CHANGED = "permission_changed"
    NETWORK_CONNECTION = "network_connection"
    ACTIVE_CHECK = "active_check"
    CUSTOM = "custom"
    # System metric triggers. Handled by the checkpoint evaluator but, until
    # recently, rejected by the Go template validator -- so no template could
    # use them and this enum never listed them.
    DISK_USAGE = "disk_usage"
    CPU_LOAD = "cpu_load"
    MEMORY_USAGE = "memory_usage"
    PROCESS_RUNNING = "process_running"
    PORT_LISTENING = "port_listening"
    CRON_JOB = "cron_job"
    FIREWALL_RULE = "firewall_rule"


class CheckpointStatus(str, Enum):
    PENDING = "pending"
    PASSED = "passed"
    FAILED = "failed"
    SKIPPED = "skipped"
    PARTIAL = "partial"


# -----------------------------------------------------------------------------
# Base Specs
# -----------------------------------------------------------------------------

class ResourceSpec(BaseModel):
    cpu: int = Field(ge=1, le=64)
    memory: int = Field(ge=512)
    disk: int | None = Field(default=None, ge=1)


class VMNetworkSpec(BaseModel):
    segment: str
    ip: str | None = None


class SnapshotSpec(BaseModel):
    name: str = Field(min_length=1, max_length=64)
    description: str | None = None
    default: bool = False
    include_ram: bool = Field(default=False, alias="includeRAM")


class AgentConfig(BaseModel):
    """Wazuh agent configuration for a VM."""
    monitor_paths: list[str] = Field(default_factory=list)
    audit_commands: bool = False
    audit_syscalls: bool = False
    realtime: bool = True


class VMSpec(BaseModel):
    name: str = Field(min_length=1, max_length=64)
    template: str
    resources: ResourceSpec
    networks: list[VMNetworkSpec] = Field(default_factory=list)
    snapshots: list[SnapshotSpec] = Field(default_factory=list)
    start_on_create: bool = Field(default=True, alias="startOnCreate")
    wazuh_agent: bool = Field(default=False, alias="wazuhAgent")
    agent_config: AgentConfig | None = Field(default=None, alias="agentConfig")


class NetworkSegment(BaseModel):
    name: str = Field(min_length=1, max_length=64)
    # Optional, mirroring the Go validator, which range-checks the VLAN only
    # when it is non-zero and so treats an absent one as valid.
    vlan: int | None = Field(default=None, ge=1, le=4094)
    subnet: str
    gateway: str | None = None
    dhcp: bool = False


class NetworkSpec(BaseModel):
    segments: list[NetworkSegment] = Field(min_length=1)


# -----------------------------------------------------------------------------
# Checkpoint System Types
# -----------------------------------------------------------------------------

class TriggerMatch(BaseModel):
    """Matching criteria for checkpoint triggers."""
    # File-based triggers
    path: str | None = None
    contains: str | None = None
    regex: str | None = None
    mode: str | None = None
    owner: str | None = None
    group: str | None = None
    permission: str | None = None  # e.g. "0644", "rw-r--r--"

    # Package triggers
    package: str | None = None
    state: str | None = None  # installed, removed, active, inactive

    # Service triggers
    name: str | None = None

    # Command triggers
    pattern: str | None = None
    user: str | None = None

    # Network triggers
    protocol: str | None = None
    destination: str | None = None
    address: str | None = None  # alias for destination
    port: int | None = None

    # User triggers
    username: str | None = None
    uid: int | None = None
    groups: list[str] | None = None

    # System metric triggers
    mount_point: str | None = None
    threshold_pct: float | None = None
    threshold_value: float | None = None
    operator: str | None = None  # lt, le, eq, ge, gt
    process_name: str | None = None
    process_count: int | None = None
    listen_address: str | None = None

    # Firewall triggers
    action: str | None = None  # accept, drop, reject
    chain: str | None = None  # INPUT, OUTPUT, FORWARD
    interface: str | None = None

    # Cron triggers
    schedule: str | None = None
    command: str | None = None  # cron command pattern -- NOT command_executed,
    # which matches on `pattern`

    model_config = {"extra": "forbid"}


# Trigger types that need no match block. active_check runs a script and is
# graded on its exit code (checkpoint/active.go); custom is open-ended.
MATCH_OPTIONAL_TRIGGERS = frozenset({TriggerType.ACTIVE_CHECK, TriggerType.CUSTOM})


class CheckpointTrigger(BaseModel):
    """A single trigger condition for a checkpoint."""
    type: TriggerType
    target: str  # VM name
    # Defaulted rather than required, because models.CheckpointTrigger.Match is
    # a value type in Go and an absent `match` unmarshals to an empty struct.
    # See validate_match_present for why an empty one is still rejected for
    # most trigger types.
    match: TriggerMatch = Field(default_factory=TriggerMatch)
    script: str | None = None  # For active_check type

    @model_validator(mode="after")
    def validate_match_present(self) -> CheckpointTrigger:
        """Require a match block for every trigger type that is graded by one.

        Deliberately stricter than Go, which cannot express this: its evaluator
        treats each match field as a *constraint to apply if set*, so an empty
        match is not "matches nothing" but "matches anything". A file_exists
        trigger with no match returns true for any added-or-modified file event
        on the target VM (evaluator.go:796 skips the path guard, then :816
        returns true), so the student banks the points for creating any file at
        all. Rejecting it here can only catch templates that are already broken.
        """
        if self.type in MATCH_OPTIONAL_TRIGGERS:
            return self
        if self.match == TriggerMatch():
            raise ValueError(
                f"Trigger of type '{self.type.value}' needs a match block; "
                f"without one it matches any event on '{self.target}'"
            )
        return self


class HintLevel(BaseModel):
    """One step of a progressive hint ladder.

    Mirrors models.HintLevel in api/internal/models/question.go. Revealing a
    hint costs the student `penalty` points, which is why dropping these on
    the floor silently was a real loss: 90 of them ship across 18 templates.
    """

    level: int = Field(ge=1)  # 1, 2, 3...
    text: str
    penalty: int = Field(default=0, ge=0)  # points deducted when revealed


class Checkpoint(BaseModel):
    """A graded checkpoint/objective in a lab."""
    id: str = Field(min_length=1, max_length=64)
    description: str
    points: int = Field(ge=0)
    # `hint` is the legacy single-hint form; `hints` is the progressive ladder
    # Go reads via Checkpoint.GetHintForLevel. Both are accepted, matching Go.
    hint: str | None = None
    hints: list[HintLevel] = Field(default_factory=list)
    depends_on: list[str] = Field(default_factory=list)
    timeout_minutes: int | None = None
    required: bool = False
    order: int | None = None
    triggers: list[CheckpointTrigger] = Field(min_length=1)

    @model_validator(mode="after")
    def validate_hint_levels(self) -> Checkpoint:
        """Levels must be unique: GetHintForLevel returns the first match."""
        levels = [h.level for h in self.hints]
        if len(levels) != len(set(levels)):
            raise ValueError(
                f"Checkpoint '{self.id}' has duplicate hint levels: {sorted(levels)}"
            )
        return self


class CheckpointConfig(BaseModel):
    """Overall checkpoint configuration for a lab."""
    enabled: bool = True
    pass_threshold: int = Field(default=70, ge=0, le=100)  # Percentage to pass
    allow_retry: bool = True
    show_hints: bool = True
    realtime_update: bool = True
    grade_on_submit: bool = False


# -----------------------------------------------------------------------------
# Lab Template Types
# -----------------------------------------------------------------------------

class InstructionResource(BaseModel):
    """A link offered alongside a lab. Mirrors models.InstructionResource."""

    title: str
    url: str


class InstructionStep(BaseModel):
    """One step of the student-facing walkthrough.

    Mirrors models.InstructionStep. `objective_id` optionally ties the step to
    a checkpoint, which is what lets the UI show progress against the text.
    """

    id: str
    title: str
    objective_id: str | None = None
    content: str


class LabInstructions(BaseModel):
    """The educational body of a lab. Mirrors models.LabInstructions.

    This is the whole student-facing content of a template. Go persists it to
    lab_templates.instructions and serves it from GET /labs/{labID}/instructions;
    this model existing means Python tooling can finally see it.
    """

    overview: str | None = None
    learning_objectives: list[str] = Field(default_factory=list)
    prerequisites: list[str] = Field(default_factory=list)
    steps: list[InstructionStep] = Field(default_factory=list)
    summary: str | None = None
    tips: list[str] = Field(default_factory=list)
    resources: list[InstructionResource] = Field(default_factory=list)


class LabMetadata(BaseModel):
    name: str = Field(min_length=1, max_length=128)
    description: str
    duration: str
    difficulty: Difficulty
    tags: list[str] = Field(default_factory=list)
    version: str = Field(default="1.0.0")
    author: str | None = None
    # Name of a parent template to inherit from. See the note on LabTemplate:
    # when set, this document is a fragment and is only fully valid once merged
    # with its parent.
    extends: str | None = None
    # Substituted into the spec by the Go loader. map[string]string in Go.
    variables: dict[str, str] = Field(default_factory=dict)
    # The student-facing walkthrough. Modelled here because it was previously
    # dropped on the floor: 36 of the shipped templates carry it, 216 steps in
    # total, and pydantic's default extra="ignore" discarded every one while
    # still reporting the template as parsed.
    instructions: LabInstructions | None = None


class LabSpec(BaseModel):
    platform: Platform = Platform.PROXMOX
    # network and vms are only required of a standalone template. A child
    # template (metadata.extends set) inherits both from its parent, so the
    # requirement is enforced on LabTemplate, which can see the metadata.
    network: NetworkSpec | None = None
    vms: list[VMSpec] = Field(default_factory=list)
    checkpoints: CheckpointConfig | None = None
    objectives: list[Checkpoint] = Field(default_factory=list)

    @model_validator(mode="after")
    def validate_self_dependencies(self) -> LabSpec:
        """A checkpoint may never depend on itself, merged or not."""
        for obj in self.objectives:
            if obj.id in obj.depends_on:
                raise ValueError(
                    f"Checkpoint '{obj.id}' cannot depend on itself"
                )
        return self


class LabTemplate(BaseModel):
    api_version: str = Field(alias="apiVersion", default=CURRENT_API_VERSION)
    # Required and checked, matching the Go validator. Without this, a YAML
    # document of some other kind -- a labtest simulator scenario, say -- picks
    # up a default of "LabTemplate" and then fails with a confusing complaint
    # about a missing `metadata` instead of "this isn't a lab template".
    kind: str
    metadata: LabMetadata
    spec: LabSpec

    model_config = {"populate_by_name": True}

    @field_validator("kind")
    @classmethod
    def validate_kind(cls, v: str) -> str:
        if v != "LabTemplate":
            raise ValueError(f"Unsupported kind '{v}'; expected 'LabTemplate'")
        return v

    @field_validator("api_version")
    @classmethod
    def validate_api_version(cls, v: str) -> str:
        """Accept the current version plus any retained legacy spelling."""
        if v not in SUPPORTED_API_VERSIONS:
            supported = ", ".join(sorted(SUPPORTED_API_VERSIONS))
            raise ValueError(
                f"Unsupported apiVersion '{v}'. Supported values: {supported}"
            )
        return v

    @property
    def is_fragment(self) -> bool:
        """True when this template inherits from a parent.

        A fragment is deliberately incomplete: network, VMs and cross-references
        may all resolve against the parent (merged by
        api/internal/templates/inheritance.go) rather than against this
        document.

        Note this is deliberately *looser* than Go, not parity with it. Go's
        loader validates before resolving inheritance
        (api/internal/templates/loader.go:140), and its validator has no
        fragment awareness -- it requires at least one VM and resolves every
        trigger target against the child's own VM list, unconditionally. So it
        produces spurious errors for a valid child template;
        templates/examples/child-security-lab.yaml is one. Those errors are
        only fatal under WithStrictMode, which nothing currently enables, so in
        practice the loader logs them and carries on. Still a Go-side gap, and
        the fix belongs there: validate the merged result, not the fragment.
        """
        return self.metadata.extends is not None

    @model_validator(mode="after")
    def validate_completeness(self) -> LabTemplate:
        """Require a standalone template to be self-contained."""
        if self.is_fragment:
            return self
        if self.spec.network is None:
            raise ValueError(
                "spec.network is required unless the template sets metadata.extends"
            )
        if not self.spec.vms:
            raise ValueError(
                "spec.vms must declare at least one VM unless the template "
                "sets metadata.extends"
            )
        return self

    @model_validator(mode="after")
    def validate_cross_references(self) -> LabTemplate:
        """Check checkpoint references against the VMs and objectives declared here.

        Skipped for fragments: a child's triggers routinely target a parent's
        VMs, and its objectives may depend on a parent's checkpoints. Those only
        resolve after the merge.
        """
        if self.is_fragment:
            return self

        vm_names = {vm.name for vm in self.spec.vms}
        checkpoint_ids = {obj.id for obj in self.spec.objectives}
        for obj in self.spec.objectives:
            for trigger in obj.triggers:
                if trigger.target not in vm_names:
                    raise ValueError(
                        f"Checkpoint '{obj.id}' trigger references unknown VM "
                        f"'{trigger.target}'. Valid VMs: {vm_names}"
                    )
            for dep in obj.depends_on:
                if dep not in checkpoint_ids:
                    raise ValueError(
                        f"Checkpoint '{obj.id}' depends on unknown checkpoint '{dep}'"
                    )
        return self


# -----------------------------------------------------------------------------
# Event Types (for Wazuh integration)
# -----------------------------------------------------------------------------

class WazuhEvent(BaseModel):
    """An event received from a Wazuh agent."""
    id: str
    timestamp: datetime
    agent_id: str
    agent_name: str
    rule_id: int
    rule_level: int
    description: str
    groups: list[str] = Field(default_factory=list)
    location: str
    decoder: str | None = None
    data: dict[str, Any] = Field(default_factory=dict)


class SyscheckData(BaseModel):
    """Wazuh file integrity monitoring data."""
    path: str
    event: str  # added, modified, deleted
    mode: str | None = None
    size: int | None = None
    md5: str | None = None
    sha1: str | None = None
    sha256: str | None = None
    uid: str | None = None
    gid: str | None = None
    owner: str | None = None
    group: str | None = None
    changed_attributes: list[str] = Field(default_factory=list)
    old_content: str | None = None
    new_content: str | None = None
    diff: str | None = None


class AuditData(BaseModel):
    """Wazuh audit log data (command execution)."""
    type: str
    command: str | None = None
    exe: str | None = None
    success: str | None = None
    uid: str | None = None
    auid: str | None = None
    euid: str | None = None
    user: str | None = None
    cwd: str | None = None


class PackageData(BaseModel):
    """Package installation/removal data."""
    package: str
    version: str | None = None
    action: str  # install, remove, upgrade
    manager: str | None = None  # apt, yum, dnf


class ServiceData(BaseModel):
    """Systemd/init service state changes."""
    unit: str
    state: str  # active, inactive, failed
    sub_state: str | None = None
    pid: int | None = None


class UserData(BaseModel):
    """User account changes."""
    username: str
    uid: int | None = None
    gid: int | None = None
    home: str | None = None
    shell: str | None = None
    groups: list[str] = Field(default_factory=list)
    action: str  # created, modified, deleted


# -----------------------------------------------------------------------------
# Session and Progress Tracking
# -----------------------------------------------------------------------------

class CheckpointState(BaseModel):
    """Current state of a checkpoint in a session."""
    checkpoint_id: str
    status: CheckpointStatus = CheckpointStatus.PENDING
    points: int
    earned_points: int = 0
    passed_at: datetime | None = None
    triggered_by: str | None = None  # Event ID
    attempt_count: int = 0
    last_attempt_at: datetime | None = None
    feedback: str | None = None


class LabSession(BaseModel):
    """An active lab session for a user."""
    id: str
    pod_id: str
    user_id: str
    lab_template: str
    canvas_assignment_id: str | None = None
    canvas_submission_id: str | None = None
    started_at: datetime
    ended_at: datetime | None = None
    due_at: datetime | None = None
    checkpoints: list[CheckpointState] = Field(default_factory=list)
    total_points: int = 0
    earned_points: int = 0
    max_points: int = 0
    percentage: float = 0.0
    grade_synced_at: datetime | None = None


class GradeReport(BaseModel):
    """Final grade report for Canvas integration."""
    session_id: str
    user_id: str
    lab_template: str
    total_points: int
    max_points: int
    percentage: float
    passed: bool
    completed_at: datetime
    duration_seconds: int
    checkpoints: list[CheckpointState]
    comments: str | None = None


def parse_template(content: str) -> LabTemplate:
    """Parse a lab template from YAML string."""
    data = yaml.safe_load(content)
    return LabTemplate.model_validate(data)


def parse_template_file(path: Path | str) -> LabTemplate:
    """Parse a lab template from a YAML file."""
    path = Path(path)
    if not path.exists():
        raise FileNotFoundError(f"Template file not found: {path}")
    return parse_template(path.read_text())


def validate_template(content: str) -> list[str]:
    """Validate a template and return a list of issues."""
    issues: list[str] = []
    try:
        template = parse_template(content)

        vm_names = [vm.name for vm in template.spec.vms]
        if len(vm_names) != len(set(vm_names)):
            issues.append("Duplicate VM names found")

        if template.spec.network is not None:
            seg_names = [s.name for s in template.spec.network.segments]
            if len(seg_names) != len(set(seg_names)):
                issues.append("Duplicate network segment names found")

        # Validate checkpoint IDs are unique
        if template.spec.objectives:
            checkpoint_ids = [obj.id for obj in template.spec.objectives]
            if len(checkpoint_ids) != len(set(checkpoint_ids)):
                issues.append("Duplicate checkpoint IDs found")

            # Check for circular dependencies
            issues.extend(_check_circular_dependencies(template.spec.objectives))

            # Validate VMs with wazuh_agent=true exist for checkpoints.
            # Skipped for a fragment: its triggers may target VMs the parent
            # declares, so agent coverage is only knowable after the merge.
            if not template.is_fragment:
                vms_with_agent = {vm.name for vm in template.spec.vms if vm.wazuh_agent}
                for obj in template.spec.objectives:
                    for trigger in obj.triggers:
                        if trigger.target not in vms_with_agent:
                            issues.append(
                                f"Checkpoint '{obj.id}' targets VM '{trigger.target}' "
                                f"which doesn't have wazuhAgent enabled"
                            )

    except (ValidationError, yaml.YAMLError) as e:
        issues.append(f"Validation error: {e}")

    return issues


def _check_circular_dependencies(objectives: list[Checkpoint]) -> list[str]:
    """Check for circular dependencies in checkpoints."""
    issues: list[str] = []
    checkpoint_map = {obj.id: obj for obj in objectives}

    def has_cycle(checkpoint_id: str, visited: set[str], path: set[str]) -> bool:
        if checkpoint_id in path:
            return True
        if checkpoint_id in visited:
            return False

        visited.add(checkpoint_id)
        path.add(checkpoint_id)

        checkpoint = checkpoint_map.get(checkpoint_id)
        if checkpoint:
            for dep in checkpoint.depends_on:
                if has_cycle(dep, visited, path):
                    return True

        path.remove(checkpoint_id)
        return False

    for obj in objectives:
        if has_cycle(obj.id, set(), set()):
            issues.append(f"Circular dependency detected involving checkpoint '{obj.id}'")
            break

    return issues


def calculate_max_points(template: LabTemplate) -> int:
    """Calculate the maximum possible points for a lab template."""
    return sum(obj.points for obj in template.spec.objectives)


if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("Usage: python parser.py <template.yaml>")
        sys.exit(1)

    template_path = Path(sys.argv[1])
    try:
        template = parse_template_file(template_path)
        print(f"✓ Template '{template.metadata.name}' is valid")
        print(f"  Platform: {template.spec.platform.value}")
        if template.is_fragment:
            print(f"  Extends: {template.metadata.extends} (fragment)")
        print(f"  VMs: {len(template.spec.vms)}")
        segments = template.spec.network.segments if template.spec.network else []
        print(f"  Networks: {len(segments)}")

        if template.spec.objectives:
            max_pts = calculate_max_points(template)
            print(f"  Checkpoints: {len(template.spec.objectives)}")
            print(f"  Max Points: {max_pts}")
            if template.spec.checkpoints:
                print(f"  Pass Threshold: {template.spec.checkpoints.pass_threshold}%")

            # Show checkpoint summary
            print("\n  Objectives:")
            for obj in template.spec.objectives:
                deps = f" (requires: {', '.join(obj.depends_on)})" if obj.depends_on else ""
                print(f"    - [{obj.points}pts] {obj.id}: {obj.description}{deps}")

        # Run full validation
        issues = validate_template(template_path.read_text())
        if issues:
            print("\n⚠ Warnings:")
            for issue in issues:
                print(f"  - {issue}")

    except Exception as e:
        print(f"✗ Validation failed: {e}")
        sys.exit(1)
