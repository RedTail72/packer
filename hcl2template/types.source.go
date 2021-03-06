package hcl2template

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/packer/packer"
)

// SourceBlock references an HCL 'source' block.
type SourceBlock struct {
	// Type of source; ex: virtualbox-iso
	Type string
	// Given name; if any
	Name string

	block *hcl.Block
}

func (p *Parser) decodeSource(block *hcl.Block) (*SourceBlock, hcl.Diagnostics) {
	source := &SourceBlock{
		Type:  block.Labels[0],
		Name:  block.Labels[1],
		block: block,
	}
	var diags hcl.Diagnostics

	if !p.BuilderSchemas.Has(source.Type) {
		diags = append(diags, &hcl.Diagnostic{
			Summary:  "Unknown " + buildSourceLabel + " type " + source.Type,
			Subject:  block.LabelRanges[0].Ptr(),
			Detail:   fmt.Sprintf("known builders: %v", p.BuilderSchemas.List()),
			Severity: hcl.DiagError,
		})
		return nil, diags
	}

	return source, diags
}

func (cfg *PackerConfig) startBuilder(source *SourceBlock, ectx *hcl.EvalContext, opts packer.GetBuildsOptions) (packer.Builder, hcl.Diagnostics, []string) {
	var diags hcl.Diagnostics

	builder, err := cfg.builderSchemas.Start(source.Type)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Summary: "Failed to load " + sourceLabel + " type",
			Detail:  err.Error(),
			Subject: &source.block.LabelRanges[0],
		})
		return builder, diags, nil
	}

	decoded, moreDiags := decodeHCL2Spec(source.block.Body, ectx, builder)
	diags = append(diags, moreDiags...)
	if moreDiags.HasErrors() {
		return nil, diags, nil
	}

	// Note: HCL prepares inside of the Start func, but Json does not. Json
	// builds are instead prepared only in command/build.go
	// TODO: either make json prepare when plugins are loaded, or make HCL
	// prepare at a later step, to make builds from different template types
	// easier to reason about.
	builderVars := source.builderVariables()
	builderVars["packer_debug"] = strconv.FormatBool(opts.Debug)
	builderVars["packer_force"] = strconv.FormatBool(opts.Force)
	builderVars["packer_on_error"] = opts.OnError

	generatedVars, warning, err := builder.Prepare(builderVars, decoded)
	moreDiags = warningErrorsToDiags(source.block, warning, err)
	diags = append(diags, moreDiags...)
	return builder, diags, generatedVars
}

// These variables will populate the PackerConfig inside of the builders.
func (source *SourceBlock) builderVariables() map[string]string {
	return map[string]string{
		"packer_build_name":   source.Name,
		"packer_builder_type": source.Type,
	}
}

func (source *SourceBlock) Ref() SourceRef {
	return SourceRef{
		Type: source.Type,
		Name: source.Name,
	}
}

type SourceRef struct {
	Type string
	Name string
}

// NoSource is the zero value of sourceRef, representing the absense of an
// source.
var NoSource SourceRef

func (r SourceRef) String() string {
	return fmt.Sprintf("%s.%s", r.Type, r.Name)
}
