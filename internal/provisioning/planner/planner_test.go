package planner

import (
	"strings"
	"testing"

	"tamestack/internal/provisioning/inventory"
	"tamestack/internal/provisioning/models"
)

func TestGeneratePlan_NginxAndLowResourceWarnings(t *testing.T) {
	inv := &inventory.ServerInventory{
		Distro:         "ubuntu",
		Family:         "debian",
		Version:        "24.04",
		Arch:           "amd64",
		RAMTotalMB:     256,
		DiskFreeGB:     1,
		InstalledWeb:   []string{"apache"},
		ListeningPorts: []int{80},
	}
	bp := models.Blueprint{
		Profile: "lemp",
		Webserver: models.WebserverSpec{
			Type: "nginx",
		},
		Database: models.DatabaseSpec{
			Type: "mariadb",
		},
	}

	plan, err := GeneratePlan("srv-1", inv, bp)
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}
	if plan.SSHPortPreserved != 22 {
		t.Errorf("SSHPortPreserved = %d, want 22", plan.SSHPortPreserved)
	}
	if len(plan.PackagesToInstall) == 0 || plan.PackagesToInstall[0] != "nginx" {
		t.Errorf("PackagesToInstall = %v, want nginx first", plan.PackagesToInstall)
	}
	if len(plan.PotentialConflicts) == 0 {
		t.Errorf("expected port 80 conflict warning when apache is installed and listening")
	}
	joinedWarnings := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "256") || !strings.Contains(joinedWarnings, "1 ГБ") {
		t.Errorf("expected low RAM and low disk warnings, got: %v", plan.Warnings)
	}
}
