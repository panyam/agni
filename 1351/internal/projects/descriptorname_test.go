package projects

import (
	"testing"

	"github.com/panyam/agni/service"
)

// TestServiceNamesTheSameProjectDescriptor holds service's copy of the descriptor name to this
// package's. ListDesignFiles hands the name to a browser. With the wrong name, the browser gets a
// design with no project and analyses it against the built-in config with nothing saying so.
func TestServiceNamesTheSameProjectDescriptor(t *testing.T) {
	if service.ProjectDescriptorName != ProjectDescriptor {
		t.Errorf("service.ProjectDescriptorName = %q, projects.ProjectDescriptor = %q", service.ProjectDescriptorName, ProjectDescriptor)
	}
}

func TestServiceNamesTheSameDesignDescriptor(t *testing.T) {
	if service.DesignDescriptorName != DesignDescriptor {
		t.Errorf("service.DesignDescriptorName = %q, projects.DesignDescriptor = %q", service.DesignDescriptorName, DesignDescriptor)
	}
}
