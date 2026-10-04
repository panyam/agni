package projects

import (
	"testing"

	"github.com/panyam/agni/service"
)

// TestServiceNamesTheSameProjectDescriptor holds service's copy of the descriptor name to this
// package's. ListDesignFiles hands it to a browser, and a browser handed the wrong name gets a
// design with no project and analyses it against the built-in config with nothing saying so.
func TestServiceNamesTheSameProjectDescriptor(t *testing.T) {
	if service.ProjectDescriptorName != ProjectDescriptor {
		t.Errorf("service.ProjectDescriptorName = %q, projects.ProjectDescriptor = %q", service.ProjectDescriptorName, ProjectDescriptor)
	}
}
