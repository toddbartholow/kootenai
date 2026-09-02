package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// LabCommands handles lab template commands
type LabCommands struct {
	client       *Client
	templatesDir string
}

// NewLabCommands creates lab command handler
func NewLabCommands(client *Client, templatesDir string) *LabCommands {
	return &LabCommands{
		client:       client,
		templatesDir: templatesDir,
	}
}

// List lists all available lab templates
func (l *LabCommands) List(ctx context.Context) error {
	templates, err := l.loadLocalTemplates()
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	if len(templates) == 0 {
		fmt.Println(T("labctl.lab.list.empty", map[string]any{"Dir": l.templatesDir}))
		return nil
	}

	// Column headers are locale-aware; the underline row is generated from
	// the rendered header widths so the rule matches even when translations
	// change header length. tabwriter handles column alignment.
	headers := []string{
		T("labctl.lab.list.colName", nil),
		T("labctl.lab.list.colPlatform", nil),
		T("labctl.lab.list.colDifficulty", nil),
		T("labctl.lab.list.colCheckpoints", nil),
		T("labctl.lab.list.colPoints", nil),
		T("labctl.lab.list.colDescription", nil),
	}
	dashes := make([]string, len(headers))
	for i, h := range headers {
		dashes[i] = strings.Repeat("-", runewidth(h))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dashes, "\t"))

	for _, t := range templates {
		checkpoints := len(t.Spec.Objectives)
		maxPoints := 0
		for _, obj := range t.Spec.Objectives {
			maxPoints += obj.Points
		}

		desc := t.Metadata.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
			t.Metadata.Name,
			t.Spec.Platform,
			t.Metadata.Difficulty,
			checkpoints,
			maxPoints,
			desc,
		)
	}
	w.Flush()

	return nil
}

// runewidth counts visible runes, not bytes. Spanish headers like
// "DIFICULTAD" are ASCII but future locales (e.g. "DESCRIPCIÓN" — already
// multi-byte in UTF-8, though single width per rune) need rune counting so
// the underline row matches the header's rendered width in the terminal.
// Kept naive (one column per rune) — full East-Asian-width handling would
// pull in golang.org/x/text/width; premature for the current locale set.
func runewidth(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// Show displays detailed information about a lab template.
// All chrome (field labels, section headings, diagnostic formatting) is
// localizable; identifiers that come from the YAML (VM names, template
// names, segment names, IP addresses) render verbatim.
func (l *LabCommands) Show(ctx context.Context, name string) error {
	template, err := l.findTemplate(name)
	if err != nil {
		return err
	}

	fmt.Println(T("labctl.lab.show.templateHeading", map[string]any{"Name": template.Metadata.Name}))
	// Horizontal rule is visual chrome, not prose — kept as a literal.
	fmt.Printf("================================================================================\n\n")

	fmt.Println(T("labctl.lab.show.fieldDescription", map[string]any{"Value": template.Metadata.Description}))
	fmt.Println(T("labctl.lab.show.fieldPlatform", map[string]any{"Value": template.Spec.Platform}))
	fmt.Println(T("labctl.lab.show.fieldDifficulty", map[string]any{"Value": template.Metadata.Difficulty}))
	fmt.Println(T("labctl.lab.show.fieldDuration", map[string]any{"Value": template.Metadata.Duration}))

	if template.Metadata.Version != "" {
		fmt.Println(T("labctl.lab.show.fieldVersion", map[string]any{"Value": template.Metadata.Version}))
	}
	if template.Metadata.Author != "" {
		fmt.Println(T("labctl.lab.show.fieldAuthor", map[string]any{"Value": template.Metadata.Author}))
	}

	if len(template.Metadata.Tags) > 0 {
		fmt.Println(T("labctl.lab.show.fieldTags", map[string]any{"Value": strings.Join(template.Metadata.Tags, ", ")}))
	}

	fmt.Println()
	fmt.Println(T("labctl.lab.show.networkHeading", nil))
	for _, seg := range template.Spec.Network.Segments {
		dhcp := ""
		if seg.DHCP {
			dhcp = T("labctl.lab.show.networkDhcpSuffix", nil)
		}
		fmt.Println(T("labctl.lab.show.networkLine", map[string]any{
			"Name": seg.Name, "Vlan": seg.VLAN, "Subnet": seg.Subnet, "Dhcp": dhcp,
		}))
	}

	fmt.Println()
	fmt.Println(T("labctl.lab.show.vmsHeading", nil))
	for _, vm := range template.Spec.VMs {
		wazuh := ""
		if vm.WazuhAgent {
			wazuh = T("labctl.lab.show.vmWazuhSuffix", nil)
		}
		fmt.Println(T("labctl.lab.show.vmName", map[string]any{"Name": vm.Name}))
		fmt.Println(T("labctl.lab.show.vmTemplate", map[string]any{"Template": vm.Template, "Wazuh": wazuh}))
		// Resources line: the base always applies; disk is an optional suffix
		// so it can flow off the same line without an extra template.
		base := T("labctl.lab.show.vmResources", map[string]any{"Cpu": vm.Resources.CPU, "Memory": vm.Resources.Memory})
		if vm.Resources.Disk > 0 {
			base += T("labctl.lab.show.vmResourcesDiskSuffix", map[string]any{"Disk": vm.Resources.Disk})
		}
		fmt.Println(base)

		for _, net := range vm.Networks {
			ip := T("labctl.lab.show.vmNetworkDhcp", nil)
			if net.IP != "" {
				ip = net.IP
			}
			fmt.Println(T("labctl.lab.show.vmNetwork", map[string]any{"Segment": net.Segment, "Ip": ip}))
		}

		if len(vm.Snapshots) > 0 {
			fmt.Print(T("labctl.lab.show.vmSnapshotsPrefix", nil))
			snapNames := make([]string, len(vm.Snapshots))
			for i, s := range vm.Snapshots {
				snapNames[i] = s.Name
				if s.Default {
					snapNames[i] += "*"
				}
			}
			fmt.Println(strings.Join(snapNames, ", "))
		}
	}

	if len(template.Spec.Objectives) > 0 {
		maxPoints := 0
		for _, obj := range template.Spec.Objectives {
			maxPoints += obj.Points
		}

		fmt.Println()
		fmt.Println(T("labctl.lab.show.checkpointsHeading", map[string]any{
			"Count": len(template.Spec.Objectives), "Max": maxPoints,
		}))
		if template.Spec.Checkpoints != nil {
			fmt.Println(T("labctl.lab.show.passThreshold", map[string]any{"Pct": template.Spec.Checkpoints.PassThreshold}))
			if template.Spec.Checkpoints.ShowHints {
				fmt.Println(T("labctl.lab.show.hintsEnabled", nil))
			}
		}

		fmt.Println()
		for i, obj := range template.Spec.Objectives {
			deps := ""
			if len(obj.DependsOn) > 0 {
				deps = T("labctl.lab.show.requiresSuffix", map[string]any{"Deps": strings.Join(obj.DependsOn, ", ")})
			}
			required := ""
			if obj.Required {
				required = T("labctl.lab.show.requiredSuffix", nil)
			}
			fmt.Println(T("labctl.lab.show.objectiveLine", map[string]any{
				"Number": i + 1, "Points": obj.Points, "Description": obj.Description,
				"Required": required, "Requires": deps,
			}))

			if len(obj.Triggers) > 0 {
				for _, t := range obj.Triggers {
					fmt.Println(T("labctl.lab.show.triggerLine", map[string]any{"Type": t.Type, "Target": t.Target}))
				}
			}
		}
	}

	return nil
}

// Validate validates a lab template file
func (l *LabCommands) Validate(ctx context.Context, path string) error {
	// #nosec G304 -- Path is CLI argument from administrator.
	// CLI is admin-only tooling, not exposed to end users.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var template models.LabTemplate
	if err := yaml.Unmarshal(data, &template); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	errors := l.validateTemplate(&template)
	if len(errors) > 0 {
		fmt.Println(T("labctl.lab.validate.errorHeading", map[string]any{"Count": len(errors)}))
		for _, e := range errors {
			fmt.Println(T("labctl.lab.validate.errorLine", map[string]any{"Err": e}))
		}
		return fmt.Errorf("validation failed")
	}

	fmt.Println(T("labctl.lab.validate.validOk", map[string]any{"Name": template.Metadata.Name}))
	fmt.Println(T("labctl.lab.validate.okPlatform", map[string]any{"Value": template.Spec.Platform}))
	fmt.Println(T("labctl.lab.validate.okVms", map[string]any{"Count": len(template.Spec.VMs)}))
	fmt.Println(T("labctl.lab.validate.okNetworks", map[string]any{"Count": len(template.Spec.Network.Segments)}))

	if len(template.Spec.Objectives) > 0 {
		maxPoints := 0
		for _, obj := range template.Spec.Objectives {
			maxPoints += obj.Points
		}
		fmt.Println(T("labctl.lab.validate.okCheckpoints", map[string]any{
			"Count": len(template.Spec.Objectives), "Max": maxPoints,
		}))
	}

	return nil
}

// validateTemplate performs validation on a template.
// Every returned string routes through the localizer so operator-facing
// diagnostics match the CLI's active locale. Field path tokens
// ("spec.vms[N].name") are preserved verbatim inside the translated
// messages — those are schema-level identifiers, not prose.
func (l *LabCommands) validateTemplate(t *models.LabTemplate) []string {
	var errors []string

	// Required fields
	if t.Metadata.Name == "" {
		errors = append(errors, T("labctl.lab.validationErrors.metadataNameRequired", nil))
	}

	if len(t.Spec.VMs) == 0 {
		errors = append(errors, T("labctl.lab.validationErrors.atLeastOneVm", nil))
	}

	// Validate VMs
	vmNames := make(map[string]bool)
	for i, vm := range t.Spec.VMs {
		if vm.Name == "" {
			errors = append(errors, T("labctl.lab.validationErrors.vmNameRequired", map[string]any{"Idx": i}))
		} else {
			if vmNames[vm.Name] {
				errors = append(errors, T("labctl.lab.validationErrors.duplicateVmName", map[string]any{"Name": vm.Name}))
			}
			vmNames[vm.Name] = true
		}

		if vm.Template == "" {
			errors = append(errors, T("labctl.lab.validationErrors.vmTemplateRequired", map[string]any{"Idx": i}))
		}

		if vm.Resources.CPU <= 0 {
			errors = append(errors, T("labctl.lab.validationErrors.cpuPositive", map[string]any{"Idx": i}))
		}
		if vm.Resources.Memory <= 0 {
			errors = append(errors, T("labctl.lab.validationErrors.memoryPositive", map[string]any{"Idx": i}))
		}
	}

	// Validate networks
	segmentNames := make(map[string]bool)
	for _, seg := range t.Spec.Network.Segments {
		if seg.Name == "" {
			errors = append(errors, T("labctl.lab.validationErrors.segmentNameRequired", nil))
		} else {
			if segmentNames[seg.Name] {
				errors = append(errors, T("labctl.lab.validationErrors.duplicateSegment", map[string]any{"Name": seg.Name}))
			}
			segmentNames[seg.Name] = true
		}
	}

	// Validate VM network references
	for _, vm := range t.Spec.VMs {
		for _, net := range vm.Networks {
			if !segmentNames[net.Segment] {
				errors = append(errors, T("labctl.lab.validationErrors.unknownSegment", map[string]any{"Vm": vm.Name, "Segment": net.Segment}))
			}
		}
	}

	// Validate checkpoints
	checkpointIDs := make(map[string]bool)
	for i, obj := range t.Spec.Objectives {
		if obj.ID == "" {
			errors = append(errors, T("labctl.lab.validationErrors.objectiveIdRequired", map[string]any{"Idx": i}))
		} else {
			if checkpointIDs[obj.ID] {
				errors = append(errors, T("labctl.lab.validationErrors.duplicateCheckpoint", map[string]any{"Id": obj.ID}))
			}
			checkpointIDs[obj.ID] = true
		}

		if obj.Points < 0 {
			errors = append(errors, T("labctl.lab.validationErrors.pointsNegative", map[string]any{"Idx": i}))
		}

		// Validate dependencies
		for _, dep := range obj.DependsOn {
			if !checkpointIDs[dep] && dep != obj.ID {
				// Might be forward reference - check all IDs
				found := false
				for _, other := range t.Spec.Objectives {
					if other.ID == dep {
						found = true
						break
					}
				}
				if !found {
					errors = append(errors, T("labctl.lab.validationErrors.unknownDep", map[string]any{"Id": obj.ID, "Dep": dep}))
				}
			}
		}

		// Validate triggers
		for j, trigger := range obj.Triggers {
			if trigger.Type == "" {
				errors = append(errors, T("labctl.lab.validationErrors.triggerTypeRequired", map[string]any{"Idx": i, "T": j}))
			}
			if trigger.Target == "" {
				errors = append(errors, T("labctl.lab.validationErrors.triggerTargetRequired", map[string]any{"Idx": i, "T": j}))
			} else if !vmNames[trigger.Target] {
				errors = append(errors, T("labctl.lab.validationErrors.triggerUnknownVm", map[string]any{"Idx": i, "T": j, "Vm": trigger.Target}))
			}
		}
	}

	return errors
}

// loadLocalTemplates loads all templates from the templates directory
func (l *LabCommands) loadLocalTemplates() ([]*models.LabTemplate, error) {
	var templates []*models.LabTemplate

	err := filepath.Walk(l.templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		// #nosec G304 -- Path comes from walking CLI-specified directory argument.
		// CLI is admin-only tooling, not exposed to end users.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		var template models.LabTemplate
		if err := yaml.Unmarshal(data, &template); err != nil {
			return nil // Skip invalid files
		}

		if template.Kind != "LabTemplate" && template.Kind != "" {
			return nil // Skip non-template files
		}

		templates = append(templates, &template)
		return nil
	})

	return templates, err
}

// findTemplate finds a template by name
func (l *LabCommands) findTemplate(name string) (*models.LabTemplate, error) {
	templates, err := l.loadLocalTemplates()
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		if t.Metadata.Name == name {
			return t, nil
		}
	}

	return nil, fmt.Errorf("template not found: %s", name)
}
