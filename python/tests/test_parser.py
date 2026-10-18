"""Tests for the lab template parser and validator.

Verifies YAML parsing, Pydantic validation, checkpoint cross-references,
circular dependency detection, and the validate_template() function.
"""

from __future__ import annotations

import re
import textwrap
from pathlib import Path

import pytest
from pydantic import ValidationError

from lab_templates.parser import (
    CURRENT_API_VERSION,
    LEGACY_API_VERSIONS,
    SUPPORTED_API_VERSIONS,
    Difficulty,
    InstructionResource,
    InstructionStep,
    LabInstructions,
    LabMetadata,
    Platform,
    TriggerMatch,
    TriggerType,
    calculate_max_points,
    parse_template,
    validate_template,
)

# ---------------------------------------------------------------------------
# Minimal valid template YAML for reuse
# ---------------------------------------------------------------------------

MINIMAL_TEMPLATE_YAML = textwrap.dedent("""\
    apiVersion: v1
    kind: LabTemplate
    metadata:
      name: Test Lab
      description: A minimal test lab
      duration: 60m
      difficulty: beginner
    spec:
      platform: proxmox
      network:
        segments:
          - name: lab-net
            vlan: 100
            subnet: 10.0.0.0/24
      vms:
        - name: lab-vm
          template: ubuntu-22.04
          resources:
            cpu: 2
            memory: 2048
""")

TEMPLATE_WITH_CHECKPOINTS_YAML = textwrap.dedent("""\
    apiVersion: v1
    kind: LabTemplate
    metadata:
      name: Checkpoint Lab
      description: Lab with checkpoints
      duration: 90m
      difficulty: intermediate
    spec:
      platform: proxmox
      network:
        segments:
          - name: lab-net
            vlan: 100
            subnet: 10.0.0.0/24
      vms:
        - name: lab-vm
          template: ubuntu-22.04
          resources:
            cpu: 2
            memory: 2048
          wazuhAgent: true
      checkpoints:
        pass_threshold: 70
      objectives:
        - id: create-file
          description: Create the report file
          points: 10
          triggers:
            - type: file_exists
              target: lab-vm
              match:
                path: /home/student/report.txt
        - id: write-content
          description: Write content to the file
          points: 20
          depends_on:
            - create-file
          triggers:
            - type: file_content
              target: lab-vm
              match:
                path: /home/student/report.txt
                contains: "Hello World"
""")


# ---------------------------------------------------------------------------
# Parsing valid templates
# ---------------------------------------------------------------------------

class TestParseValidTemplate:
    """Test parsing well-formed template YAML."""

    def test_minimal_template(self) -> None:
        template = parse_template(MINIMAL_TEMPLATE_YAML)
        assert template.metadata.name == "Test Lab"
        assert template.spec.platform == Platform.PROXMOX
        assert len(template.spec.vms) == 1
        assert template.spec.vms[0].name == "lab-vm"

    def test_template_with_checkpoints(self) -> None:
        template = parse_template(TEMPLATE_WITH_CHECKPOINTS_YAML)
        assert len(template.spec.objectives) == 2
        assert template.spec.objectives[0].id == "create-file"
        assert template.spec.objectives[1].depends_on == ["create-file"]

    def test_metadata_fields(self) -> None:
        template = parse_template(MINIMAL_TEMPLATE_YAML)
        assert template.metadata.description == "A minimal test lab"
        assert template.metadata.duration == "60m"
        assert template.metadata.difficulty == Difficulty.BEGINNER

    def test_api_version_round_trips(self) -> None:
        template = parse_template(MINIMAL_TEMPLATE_YAML)
        assert template.api_version == CURRENT_API_VERSION

    def test_vm_resources(self) -> None:
        template = parse_template(MINIMAL_TEMPLATE_YAML)
        vm = template.spec.vms[0]
        assert vm.resources.cpu == 2
        assert vm.resources.memory == 2048

    def test_network_segments(self) -> None:
        template = parse_template(MINIMAL_TEMPLATE_YAML)
        segments = template.spec.network.segments
        assert len(segments) == 1
        assert segments[0].name == "lab-net"
        assert segments[0].vlan == 100
        assert segments[0].subnet == "10.0.0.0/24"

    def test_checkpoint_config(self) -> None:
        template = parse_template(TEMPLATE_WITH_CHECKPOINTS_YAML)
        assert template.spec.checkpoints is not None
        assert template.spec.checkpoints.pass_threshold == 70

    def test_checkpoint_triggers(self) -> None:
        template = parse_template(TEMPLATE_WITH_CHECKPOINTS_YAML)
        trigger = template.spec.objectives[0].triggers[0]
        assert trigger.type.value == "file_exists"
        assert trigger.target == "lab-vm"
        assert trigger.match.path == "/home/student/report.txt"


# ---------------------------------------------------------------------------
# Validation: missing required fields
# ---------------------------------------------------------------------------

class TestMissingRequiredFields:
    """Verify that missing required fields raise ValidationError."""

    def test_missing_metadata_name(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              description: No name
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
        """)
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_missing_vms(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Test
              description: No VMs
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms: []
        """)
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_missing_network_segments(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Test
              description: No network
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments: []
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
        """)
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_missing_checkpoint_triggers(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Test
              description: Checkpoint without triggers
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
              objectives:
                - id: obj1
                  description: An objective
                  points: 10
                  triggers: []
        """)
        with pytest.raises(ValidationError):
            parse_template(yaml_str)


# ---------------------------------------------------------------------------
# Validation: invalid values
# ---------------------------------------------------------------------------

class TestInvalidValues:
    """Verify validation catches out-of-range and invalid values."""

    def test_invalid_difficulty(self) -> None:
        yaml_str = MINIMAL_TEMPLATE_YAML.replace("difficulty: beginner", "difficulty: impossible")
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_invalid_platform(self) -> None:
        yaml_str = MINIMAL_TEMPLATE_YAML.replace("platform: proxmox", "platform: vmware")
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_cpu_below_minimum(self) -> None:
        yaml_str = MINIMAL_TEMPLATE_YAML.replace("cpu: 2", "cpu: 0")
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_memory_below_minimum(self) -> None:
        yaml_str = MINIMAL_TEMPLATE_YAML.replace("memory: 2048", "memory: 128")
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_vlan_out_of_range(self) -> None:
        yaml_str = MINIMAL_TEMPLATE_YAML.replace("vlan: 100", "vlan: 5000")
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_vlan_zero(self) -> None:
        yaml_str = MINIMAL_TEMPLATE_YAML.replace("vlan: 100", "vlan: 0")
        with pytest.raises(ValidationError):
            parse_template(yaml_str)

    def test_pass_threshold_over_100(self) -> None:
        yaml_str = TEMPLATE_WITH_CHECKPOINTS_YAML.replace(
            "pass_threshold: 70", "pass_threshold: 150"
        )
        with pytest.raises(ValidationError):
            parse_template(yaml_str)


# ---------------------------------------------------------------------------
# Checkpoint cross-validation
# ---------------------------------------------------------------------------

class TestCheckpointCrossValidation:
    """Verify model validators catch checkpoint reference errors."""

    def test_trigger_references_unknown_vm(self) -> None:
        yaml_str = TEMPLATE_WITH_CHECKPOINTS_YAML.replace(
            "target: lab-vm", "target: nonexistent-vm"
        )
        with pytest.raises(ValidationError, match="unknown VM"):
            parse_template(yaml_str)

    def test_depends_on_unknown_checkpoint(self) -> None:
        yaml_str = TEMPLATE_WITH_CHECKPOINTS_YAML.replace(
            "- create-file",  # depends_on value
            "- nonexistent-checkpoint",
        )
        with pytest.raises(ValidationError, match="unknown checkpoint"):
            parse_template(yaml_str)

    def test_self_dependency_rejected(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Self Dep
              description: Test
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
              objectives:
                - id: obj1
                  description: Self-referencing
                  points: 10
                  depends_on:
                    - obj1
                  triggers:
                    - type: file_exists
                      target: vm
                      match:
                        path: /tmp/test
        """)
        with pytest.raises(ValidationError, match="cannot depend on itself"):
            parse_template(yaml_str)


# ---------------------------------------------------------------------------
# validate_template()
# ---------------------------------------------------------------------------

class TestValidateTemplate:
    """Test the validate_template() function that returns issue lists."""

    def test_valid_template_no_issues(self) -> None:
        issues = validate_template(TEMPLATE_WITH_CHECKPOINTS_YAML)
        # The only expected "issue" is the wazuhAgent check -- triggers
        # target lab-vm which has wazuhAgent=true, so should be clean
        assert issues == []

    def test_duplicate_vm_names_detected(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Dup VMs
              description: Duplicate VM names
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
        """)
        issues = validate_template(yaml_str)
        assert any("Duplicate VM names" in issue for issue in issues)

    def test_duplicate_segment_names_detected(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Dup Nets
              description: Duplicate network names
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
                  - name: net
                    vlan: 200
                    subnet: 10.0.1.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
        """)
        issues = validate_template(yaml_str)
        assert any("Duplicate network segment" in issue for issue in issues)

    def test_invalid_yaml_returns_validation_error(self) -> None:
        issues = validate_template("not: valid: yaml: {{{}}}::")
        assert len(issues) > 0
        assert any("Validation error" in issue or "error" in issue.lower() for issue in issues)

    def test_completely_broken_structure(self) -> None:
        issues = validate_template("{}")
        assert len(issues) > 0

    def test_missing_wazuh_agent_flagged(self) -> None:
        """Objectives targeting a VM without wazuhAgent should be flagged."""
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: No Agent
              description: VM without agent
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
              objectives:
                - id: obj1
                  description: Test
                  points: 10
                  triggers:
                    - type: file_exists
                      target: vm
                      match:
                        path: /tmp/test
        """)
        issues = validate_template(yaml_str)
        assert any("wazuhAgent" in issue for issue in issues)


# ---------------------------------------------------------------------------
# Circular dependency detection
# ---------------------------------------------------------------------------

class TestCircularDependencies:
    """Test the _check_circular_dependencies helper via validate_template."""

    def test_circular_dependency_detected(self) -> None:
        yaml_str = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Circular
              description: Circular deps
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: vm
                  template: ubuntu
                  resources:
                    cpu: 1
                    memory: 512
                  wazuhAgent: true
              objectives:
                - id: obj-a
                  description: A
                  points: 10
                  depends_on:
                    - obj-b
                  triggers:
                    - type: file_exists
                      target: vm
                      match:
                        path: /tmp/a
                - id: obj-b
                  description: B
                  points: 10
                  depends_on:
                    - obj-a
                  triggers:
                    - type: file_exists
                      target: vm
                      match:
                        path: /tmp/b
        """)
        issues = validate_template(yaml_str)
        assert any("Circular dependency" in issue for issue in issues)


# ---------------------------------------------------------------------------
# calculate_max_points()
# ---------------------------------------------------------------------------

class TestCalculateMaxPoints:
    """Test the max points calculation."""

    def test_no_objectives(self) -> None:
        template = parse_template(MINIMAL_TEMPLATE_YAML)
        assert calculate_max_points(template) == 0

    def test_with_objectives(self) -> None:
        template = parse_template(TEMPLATE_WITH_CHECKPOINTS_YAML)
        # create-file=10 + write-content=20
        assert calculate_max_points(template) == 30


# ---------------------------------------------------------------------------
# Enum coverage
# ---------------------------------------------------------------------------

class TestEnums:
    """Verify enum values parse correctly from YAML strings."""

    @pytest.mark.parametrize(
        "value,expected",
        [
            ("proxmox", Platform.PROXMOX),
            ("cloudstack", Platform.CLOUDSTACK),
            ("any", Platform.ANY),
        ],
    )
    def test_platform_enum(self, value: str, expected: Platform) -> None:
        assert Platform(value) == expected

    @pytest.mark.parametrize(
        "value,expected",
        [
            ("beginner", Difficulty.BEGINNER),
            ("intermediate", Difficulty.INTERMEDIATE),
            ("advanced", Difficulty.ADVANCED),
            ("expert", Difficulty.EXPERT),
        ],
    )
    def test_difficulty_enum(self, value: str, expected: Difficulty) -> None:
        assert Difficulty(value) == expected

    def test_invalid_platform_raises(self) -> None:
        with pytest.raises(ValueError):
            Platform("docker")

    def test_invalid_difficulty_raises(self) -> None:
        with pytest.raises(ValueError):
            Difficulty("nightmare")


# ---------------------------------------------------------------------------
# The shipped templates under templates/
# ---------------------------------------------------------------------------

TEMPLATES_DIR = Path(__file__).resolve().parents[2] / "templates"
_API_DIR = Path(__file__).resolve().parents[2] / "api"


def _shipped_templates() -> list[Path]:
    if not TEMPLATES_DIR.is_dir():
        return []
    # *.scenario.yaml are labtest simulator scenarios, not lab templates.
    return sorted(
        p
        for p in TEMPLATES_DIR.rglob("*.y*ml")
        if not p.name.endswith(".scenario.yaml")
    )


def test_template_corpus_is_present() -> None:
    """Floor guard: the shipped-template tests must not pass over nothing.

    TestShippedTemplates skips when templates/ is absent and would assert over
    an empty list if the directory existed but the glob stopped matching, so
    the drift-detection job could go green having checked nothing.
    """
    assert len(_shipped_templates()) >= 40


@pytest.mark.skipif(not TEMPLATES_DIR.is_dir(), reason="templates/ not present")
class TestShippedTemplates:
    """Guard the templates that ship with the platform."""

    def test_all_parse(self) -> None:
        broken = []
        for path in _shipped_templates():
            try:
                parse_template(path.read_text())
            except Exception as exc:  # noqa: BLE001 - want the name in the message
                broken.append(f"{path.relative_to(TEMPLATES_DIR)}: {exc}")
        assert not broken, "templates failed to parse:\n" + "\n".join(broken)

    def test_progressive_hints_survive_parsing(self) -> None:
        """Hints carry a point penalty, so dropping them silently loses grading data.

        The Python model had no `hints` field at all, so pydantic discarded
        every one of them while still reporting the template as parsed.
        """
        total = 0
        for path in _shipped_templates():
            template = parse_template(path.read_text())
            total += sum(len(o.hints) for o in template.spec.objectives)
        assert total >= 90, f"expected the shipped hint ladders to survive, got {total}"

    def test_instructions_survive_parsing(self) -> None:
        """The student-facing walkthrough is the largest thing a template carries.

        LabMetadata modelled neither `instructions` nor `variables`, so
        pydantic discarded 216 steps across 36 templates -- the entire body of
        every lab -- while still reporting each one as parsed.
        """
        steps = 0
        with_instructions = 0
        with_variables = 0
        for path in _shipped_templates():
            metadata = parse_template(path.read_text()).metadata
            if metadata.variables:
                with_variables += 1
            if metadata.instructions:
                with_instructions += 1
                steps += len(metadata.instructions.steps)
        assert with_instructions >= 36, f"only {with_instructions} kept instructions"
        assert steps >= 216, f"only {steps} instruction steps survived"
        assert with_variables >= 2, f"only {with_variables} kept variables"

    def test_instruction_objective_refs_resolve(self) -> None:
        """A step pointing at a checkpoint that doesn't exist shows no progress.

        Checked over the corpus rather than enforced in the model, since Go
        does not enforce it either and a fragment's step could legitimately
        reference a parent's objective.
        """
        dangling = []
        for path in _shipped_templates():
            template = parse_template(path.read_text())
            if template.is_fragment or not template.metadata.instructions:
                continue
            objective_ids = {o.id for o in template.spec.objectives}
            for step in template.metadata.instructions.steps:
                if step.objective_id and step.objective_id not in objective_ids:
                    dangling.append(
                        f"{path.relative_to(TEMPLATES_DIR)}: step '{step.id}' "
                        f"references objective '{step.objective_id}'"
                    )
        assert not dangling, "instruction steps referencing unknown objectives:\n" + "\n".join(
            dangling
        )

    def test_every_checkpoint_target_runs_an_agent(self) -> None:
        """A checkpoint targeting an agentless VM can never fire.

        file_exists / file_content / command_executed are all detected from
        Wazuh events emitted by the agent on the target VM. Without one the
        student does the work and silently earns nothing, which is why this is
        worth a test rather than an advisory.
        """
        uncovered = []
        for path in _shipped_templates():
            template = parse_template(path.read_text())
            if template.is_fragment:
                continue  # agent coverage is only knowable after the merge
            with_agent = {vm.name for vm in template.spec.vms if vm.wazuh_agent}
            for objective in template.spec.objectives:
                for trigger in objective.triggers:
                    if trigger.target not in with_agent:
                        uncovered.append(
                            f"{path.relative_to(TEMPLATES_DIR)}: checkpoint "
                            f"'{objective.id}' targets '{trigger.target}', which "
                            f"has no Wazuh agent"
                        )
        assert not uncovered, "ungradeable checkpoints:\n" + "\n".join(uncovered)


# ---------------------------------------------------------------------------
# Template inheritance (metadata.extends)
# ---------------------------------------------------------------------------

CHILD_TEMPLATE_YAML = textwrap.dedent("""\
    apiVersion: v1
    kind: LabTemplate
    metadata:
      name: Child Lab
      description: Extends a parent
      duration: 90m
      difficulty: intermediate
      extends: base-linux-lab
    spec:
      vms:
        - name: extra-vm
          template: ubuntu-22.04
          resources:
            cpu: 1
            memory: 512
      objectives:
        - id: child-obj
          description: Targets a VM the parent declares
          points: 10
          depends_on:
            - parent-obj
          triggers:
            - type: file_exists
              target: parent-vm
              match:
                path: /tmp/x
""")



def _go_struct_yaml_fields(source: Path, struct_name: str) -> set[str]:
    """Return the yaml key of every field declared on a Go struct.

    Enumerates field *declarations* rather than scraping yaml tags directly: a
    field added with only a json tag would be invisible to a tag scrape, and
    yaml.v3 still binds such a field by lowercasing its Go name -- so a template
    could set it, Go would accept it, and a Python model with extra="forbid"
    would reject it. Raising on an untagged field turns that into a named
    failure instead of a silent pass.
    """
    block = source.read_text().split(f"type {struct_name} struct {{")[1].split("\n}")[0]
    declarations = re.findall(r"^\t(\w+)\s+[\w\[\]*.]+\s+`([^`]*)`", block, re.M)
    assert declarations, f"failed to parse {struct_name} out of {source.name}"

    untagged = [name for name, tags in declarations if "yaml:" not in tags]
    assert not untagged, (
        f"{struct_name} fields with no yaml tag, which yaml.v3 still binds by "
        f"lowercased name and this cannot see: {untagged}"
    )
    return {re.search(r'yaml:"([^",]+)', tags).group(1) for _, tags in declarations}


def _standalone(
    *,
    network: bool = True,
    vms: bool = True,
    objectives: bool = True,
    target: str = "vm-a",
    depends_on: str | None = None,
) -> str:
    """Build a standalone template, optionally breaking exactly one thing."""
    parts = ["apiVersion: v1", "kind: LabTemplate", "metadata:", "  name: Standalone",
             "  description: d", "  duration: 60m", "  difficulty: beginner", "spec:"]
    if network:
        parts += ["  network:", "    segments:", "      - name: n",
                  "        subnet: 10.0.0.0/24"]
    if vms:
        parts += ["  vms:", "    - name: vm-a", "      template: ubuntu-22.04",
                  "      resources:", "        cpu: 1", "        memory: 512"]
    if objectives:
        parts += ["  objectives:", "    - id: obj-a", "      description: d",
                  "      points: 10"]
        if depends_on is not None:
            parts += ["      depends_on:", f"        - {depends_on}"]
        parts += ["      triggers:", "        - type: file_exists",
                  f"          target: {target}", "          match:",
                  "            path: /tmp/x"]
    return "\n".join(parts) + "\n"


class TestTemplateInheritance:
    """A child template is a fragment; it is only complete once merged.

    Mirrors api/internal/templates/inheritance.go, which merges child onto
    parent and validates the result rather than the fragment.
    """

    def test_child_parses_without_network(self) -> None:
        template = parse_template(CHILD_TEMPLATE_YAML)
        assert template.is_fragment
        assert template.metadata.extends == "base-linux-lab"
        assert template.spec.network is None

    def test_child_may_reference_parent_vms_and_checkpoints(self) -> None:
        """Cross-references resolve after the merge, so they aren't checked here."""
        template = parse_template(CHILD_TEMPLATE_YAML)
        trigger = template.spec.objectives[0].triggers[0]
        assert trigger.target == "parent-vm"
        assert "parent-obj" in template.spec.objectives[0].depends_on

    def test_child_still_rejects_self_dependency(self) -> None:
        """Self-dependency is invalid regardless of merging."""
        bad = CHILD_TEMPLATE_YAML.replace("- parent-obj", "- child-obj")
        with pytest.raises(ValidationError, match="cannot depend on itself"):
            parse_template(bad)

    # Each of the following violates exactly one rule. Several after-validators
    # run on LabTemplate and the first to raise short-circuits, so a fixture
    # that breaks two rules would pass for whichever reason happened to fire
    # first and would silently change meaning if the validators were reordered.

    def test_standalone_still_requires_network(self) -> None:
        with pytest.raises(ValidationError, match="spec.network is required"):
            parse_template(_standalone(network=False))

    def test_standalone_still_requires_at_least_one_vm(self) -> None:
        with pytest.raises(ValidationError, match="at least one VM"):
            parse_template(_standalone(vms=False, objectives=False))

    def test_standalone_still_rejects_unknown_vm_target(self) -> None:
        with pytest.raises(ValidationError, match="unknown VM"):
            parse_template(_standalone(target="ghost-vm"))

    def test_standalone_still_rejects_unknown_checkpoint_dependency(self) -> None:
        with pytest.raises(ValidationError, match="unknown checkpoint"):
            parse_template(_standalone(depends_on="no-such-checkpoint"))

    def test_the_standalone_fixture_is_itself_valid(self) -> None:
        """Guards the tests above: each must fail for its own reason, not a typo."""
        template = parse_template(_standalone())
        assert not template.is_fragment


# ---------------------------------------------------------------------------
# apiVersion handling
# ---------------------------------------------------------------------------

class TestApiVersion:
    """Templates may carry any supported apiVersion; unknown values are rejected."""

    @pytest.mark.parametrize("version", sorted(SUPPORTED_API_VERSIONS))
    def test_supported_versions_parse(self, version: str) -> None:
        yaml_text = MINIMAL_TEMPLATE_YAML.replace(
            f"apiVersion: {CURRENT_API_VERSION}", f"apiVersion: {version}"
        )
        assert parse_template(yaml_text).api_version == version

    def test_legacy_versions_still_parse(self) -> None:
        """Templates written before the apiVersion was canonicalised keep loading."""
        assert LEGACY_API_VERSIONS, "expected at least one retained legacy version"
        for version in LEGACY_API_VERSIONS:
            yaml_text = MINIMAL_TEMPLATE_YAML.replace(
                f"apiVersion: {CURRENT_API_VERSION}", f"apiVersion: {version}"
            )
            assert parse_template(yaml_text).api_version == version

    def test_defaults_to_current_when_omitted(self) -> None:
        yaml_text = MINIMAL_TEMPLATE_YAML.replace(
            f"apiVersion: {CURRENT_API_VERSION}\n", ""
        )
        assert parse_template(yaml_text).api_version == CURRENT_API_VERSION

    @pytest.mark.parametrize("version", ["v2", "example.com/v1", "1", ""])
    def test_unsupported_version_rejected(self, version: str) -> None:
        yaml_text = MINIMAL_TEMPLATE_YAML.replace(
            f"apiVersion: {CURRENT_API_VERSION}", f"apiVersion: '{version}'"
        )
        with pytest.raises(ValidationError, match="Unsupported apiVersion"):
            parse_template(yaml_text)

    def test_trigger_types_match_the_go_enum(self) -> None:
        """TriggerType must mirror models.AllTriggerTypes.

        These drifted before: the Go validator carried its own hand-written
        list that fell seven behind the evaluator, and this enum copied the
        short list, so templates using disk_usage and friends were rejected on
        both sides despite being implemented.
        """
        go_source = _API_DIR / "internal" / "models" / "checkpoint.go"
        if not go_source.is_file():
            pytest.skip("Go source not available")
        source = go_source.read_text()

        block = source.split("var AllTriggerTypes = []TriggerType{")[1].split("}")[0]
        # Deliberately permissive value charset. A narrow one (e.g. [a-z_]+)
        # would fail to resolve a constant whose value contained a digit or a
        # hyphen, and silently dropping it here would make this test pass on
        # exactly the drift it exists to catch.
        const_to_value = dict(re.findall(r'(TriggerType\w+)\s+TriggerType = "([^"]+)"', source))

        names = re.findall(r"(TriggerType\w+),", block)
        assert names, "failed to parse AllTriggerTypes out of the Go source"
        unresolved = [n for n in names if n not in const_to_value]
        assert not unresolved, f"could not resolve Go constants: {unresolved}"

        assert {const_to_value[n] for n in names} == {t.value for t in TriggerType}

    def test_optional_fields_match_the_go_schema(self) -> None:
        """Fields Go treats as optional must not be required here.

        The Go side is the production parser; divergence means templates it
        accepts fail to load in Python tooling.
        """
        # models.CheckpointTrigger.Match is a value type with no omitempty, so
        # an absent match is a zero struct rather than an error. Script-based
        # active_check triggers are graded on exit code (checkpoint/active.go).
        no_match = textwrap.dedent("""\
            apiVersion: v1
            kind: LabTemplate
            metadata:
              name: Active Check Lab
              description: active_check without a match block
              duration: 60m
              difficulty: beginner
            spec:
              network:
                segments:
                  - name: lab-net
                    vlan: 100
                    subnet: 10.0.0.0/24
              vms:
                - name: lab-vm
                  template: ubuntu-22.04
                  resources:
                    cpu: 1
                    memory: 512
              objectives:
                - id: check
                  description: Script decides via exit code
                  points: 10
                  triggers:
                    - type: active_check
                      target: lab-vm
                      script: "exit 0"
            """)
        template = parse_template(no_match)
        assert template.spec.objectives[0].triggers[0].match.path is None

        # validator.go range-checks the VLAN only when non-zero, so an absent
        # one is valid there.
        no_vlan = MINIMAL_TEMPLATE_YAML.replace("        vlan: 100\n", "")
        assert parse_template(no_vlan).spec.network.segments[0].vlan is None

    def test_match_required_for_event_matched_triggers(self) -> None:
        """A matchless file_exists matches any file event, so reject it.

        Go cannot express this: its evaluator applies each match field only when
        set, so an empty match widens the trigger to everything rather than
        narrowing it to nothing.
        """
        no_match = _standalone().replace(
            "          match:\n            path: /tmp/x\n", ""
        )
        with pytest.raises(ValidationError, match="needs a match block"):
            parse_template(no_match)

    def test_match_optional_for_script_graded_triggers(self) -> None:
        for trigger_type in ("active_check", "custom"):
            text = (
                _standalone()
                .replace("          match:\n            path: /tmp/x\n", "")
                .replace("        - type: file_exists", f"        - type: {trigger_type}")
            )
            assert parse_template(text).spec.objectives[0].triggers[0].match.path is None

    def test_wrong_kind_rejected_clearly(self) -> None:
        """A non-LabTemplate document should say so, not complain about metadata.

        templates/scenarios/*.scenario.yaml are labtest simulator scenarios with
        a completely different shape; pointing the parser at one should be an
        obvious error.
        """
        scenario = textwrap.dedent("""\
            kind: Scenario
            lab_template: "Incident Response"
            steps:
              - objective: preserve-evidence
                target: forensics-workstation
            """)
        with pytest.raises(ValidationError, match="Unsupported kind"):
            parse_template(scenario)

    def test_missing_kind_rejected(self) -> None:
        no_kind = MINIMAL_TEMPLATE_YAML.replace("kind: LabTemplate\n", "")
        with pytest.raises(ValidationError, match="kind"):
            parse_template(no_kind)

    def test_vlan_still_range_checked_when_present(self) -> None:
        bad = MINIMAL_TEMPLATE_YAML.replace("vlan: 100", "vlan: 9999")
        with pytest.raises(ValidationError):
            parse_template(bad)

    def test_no_shipped_template_uses_a_legacy_api_version(self) -> None:
        """virtuallab.dev belongs to the University of Groningen, not this project.

        It stays parseable so old files keep loading, but nothing we ship may
        declare it. Asserting that about the corpus is the check with teeth;
        asserting it about CURRENT_API_VERSION restates the constant.
        """
        offenders = [
            str(p.relative_to(TEMPLATES_DIR))
            for p in _shipped_templates()
            if parse_template(p.read_text()).api_version in LEGACY_API_VERSIONS
        ]
        assert not offenders, f"templates on a legacy apiVersion: {offenders}"

    def test_trigger_match_fields_match_the_go_struct(self) -> None:
        """TriggerMatch must mirror models.TriggerMatch field for field.

        This matters more than the TriggerType parity check: TriggerMatch sets
        extra="forbid", so a field Go gains and Python lacks does not degrade
        gracefully -- every shipped template using it fails to parse. Failing
        here instead names the missing field once.
        """
        go_source = _API_DIR / "internal" / "models" / "checkpoint.go"
        if not go_source.is_file():
            pytest.skip("Go source not available")
        go_fields = _go_struct_yaml_fields(go_source, "TriggerMatch")
        missing = go_fields - set(TriggerMatch.model_fields)
        extra = set(TriggerMatch.model_fields) - go_fields
        assert not missing, f"present in Go, missing from Python TriggerMatch: {sorted(missing)}"
        assert not extra, f"present in Python TriggerMatch, missing from Go: {sorted(extra)}"

    def test_lab_metadata_fields_match_the_go_struct(self) -> None:
        """LabMetadata must mirror models.LabMetadata.

        This is the gap that lost 216 instruction steps across 36 templates:
        `instructions` and `variables` existed in Go and simply were not
        modelled here, so pydantic's default extra="ignore" discarded them
        while reporting each template as parsed.
        """
        go_source = _API_DIR / "internal" / "models" / "types.go"
        if not go_source.is_file():
            pytest.skip("Go source not available")
        go_fields = _go_struct_yaml_fields(go_source, "LabMetadata")
        missing = go_fields - set(LabMetadata.model_fields)
        assert not missing, f"present in Go, missing from Python LabMetadata: {sorted(missing)}"

    def test_instruction_models_match_the_go_structs(self) -> None:
        """The instruction models must mirror Go's, field for field."""
        go_source = _API_DIR / "internal" / "models" / "types.go"
        if not go_source.is_file():
            pytest.skip("Go source not available")
        for struct_name, model in (
            ("LabInstructions", LabInstructions),
            ("InstructionStep", InstructionStep),
            ("InstructionResource", InstructionResource),
        ):
            go_fields = _go_struct_yaml_fields(go_source, struct_name)
            missing = go_fields - set(model.model_fields)
            assert not missing, (
                f"present in Go {struct_name}, missing from Python: {sorted(missing)}"
            )
