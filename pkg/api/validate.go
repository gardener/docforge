package api

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// ValidateManifest performs validation of manifest according to
// the API rules for Documentation
func ValidateManifest(manifest *Documentation) error {
	var errs *multierror.Error
	if manifest != nil {
		if manifest.NodeSelector == nil && manifest.Structure == nil {
			errs = multierror.Append(errs, fmt.Errorf("At least nodeSelector or structure must be present as top-level elements in a manifest"))
		}
		validateNodeSelector(manifest.NodeSelector, errs)
		if manifest.NodeSelector != nil {
			validateStructure(manifest.Structure, errs)
		}
	}
	return errs.ErrorOrNil()
}

func validateStructure(structure []*Node, errs *multierror.Error) {
	for _, node := range structure {
		validateNode(node, errs)
		validateStructure(node.Nodes, errs)
	}
}

func validateNode(node *Node, errs *multierror.Error) {
	if len(node.Name) == 0 {
		if len(node.Nodes) != 0 {
			errs = multierror.Append(errs, fmt.Errorf("node property name must not be nil in container nodes"))
		}
		if len(node.ContentSelectors) > 0 {
			errs = multierror.Append(errs, fmt.Errorf("node property name must not be nil in document node with contentSelectors"))
		}
		if node.Template != nil {
			errs = multierror.Append(errs, fmt.Errorf("node property name must not be nil in document node with template"))
		}
	}
	if len(node.Name) > 0 && len(node.Source) > 0 {
		if strings.Contains(node.Name, "$name") || strings.Contains(node.Name, "$uuid") || strings.Contains(node.Name, "$ext") {
			multierror.Append(errs, fmt.Errorf("node name variables are supported only together with source property: %s", node.Name))
		}
	}
	if len(node.Source) > 0 && node.ContentSelectors != nil {
		multierror.Append(errs, fmt.Errorf("node source and contentSelectors are mutually exclusive properties"))
	}
	if len(node.Source) > 0 && node.Template != nil {
		multierror.Append(errs, fmt.Errorf("node source and template are mutually exclusive properties"))
	}
	if node.ContentSelectors != nil && node.Template != nil {
		multierror.Append(errs, fmt.Errorf("node contentSelectors and template are mutually exclusive properties"))
	}
	if len(node.Nodes) != 0 && len(node.ContentSelectors) > 0 {
		multierror.Append(errs, fmt.Errorf("node nodes and contentSelectors are mutually exclusive properties"))
	}
	if len(node.Nodes) != 0 && len(node.Source) > 0 {
		multierror.Append(errs, fmt.Errorf("node nodes and source are mutually exclusive properties"))
	}
	if len(node.Nodes) != 0 && node.Template != nil {
		multierror.Append(errs, fmt.Errorf("node nodes and template are mutually exclusive properties"))
	}
	validateNodeSelector(node.NodeSelector, errs)
}

func validateNodeSelector(ns *NodeSelector, errs *multierror.Error) {
	if ns != nil {
		if len(ns.Path) == 0 {
			multierror.Append(errs, fmt.Errorf("nodeSelector path is mandatory property"))
		}
		if ns.Depth < 0 {
			multierror.Append(errs, fmt.Errorf("nodeSelector depth property must be a positive integer"))
		}
	}
}
