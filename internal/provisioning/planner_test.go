package provisioning_test

import (
	"testing"

	"tamestack/internal/provisioning/inventory"
	"tamestack/internal/provisioning/models"
	"tamestack/internal/provisioning/planner"
	"tamestack/internal/provisioning/profiles"
)

func TestProfileCatalog(t *testing.T) {
	list := profiles.ListProfiles()
	if len(list) < 6 {
		t.Errorf("expected at least 6 profiles, got %d", len(list))
	}

	bpLaravel := profiles.GetBlueprintForProfile(models.ProfileLaravel, 2222)
	if bpLaravel.Profile != models.ProfileLaravel {
		t.Errorf("got profile %s, want %s", bpLaravel.Profile, models.ProfileLaravel)
	}
	if bpLaravel.Security.AllowSSHPort != 2222 {
		t.Errorf("got AllowSSHPort=%d, want 2222", bpLaravel.Security.AllowSSHPort)
	}
	if len(bpLaravel.PHP.Versions) == 0 {
		t.Errorf("expected PHP versions in Laravel blueprint")
	}
}

func TestPlannerDebianMinimal(t *testing.T) {
	inv := &inventory.ServerInventory{
		Distro:         "ubuntu",
		Family:         "debian",
		Version:        "22.04",
		Arch:           "x86_64",
		RAMTotalMB:     2048,
		RAMFreeMB:      1500,
		DiskTotalGB:    50,
		DiskFreeGB:     40,
		SSHPort:        22,
		PackageManager: "apt",
	}

	bp := profiles.GetBlueprintForProfile(models.ProfileMinimalPHP, 22)
	plan, err := planner.GeneratePlan("srv-test", inv, bp)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.SSHPortPreserved != 22 {
		t.Errorf("got SSHPortPreserved=%d, want 22", plan.SSHPortPreserved)
	}
	if len(plan.PackagesToInstall) == 0 {
		t.Errorf("expected packages to install on clean server")
	}

	// Verify Nginx and PHP are planned
	hasNginx := false
	for _, pkg := range plan.PackagesToInstall {
		if pkg == "nginx" {
			hasNginx = true
			break
		}
	}
	if !hasNginx {
		t.Errorf("expected nginx package in plan")
	}
}

func TestPlannerIdempotency(t *testing.T) {
	inv := &inventory.ServerInventory{
		Distro:         "ubuntu",
		Family:         "debian",
		Version:        "24.04",
		Arch:           "x86_64",
		RAMTotalMB:     4096,
		DiskFreeGB:     80,
		SSHPort:        22,
		PackageManager: "apt",
		InstalledWeb:   []string{"nginx"},
		InstalledPHP:   []string{"8.3"},
		InstalledDB:    []string{"mariadb"},
		HasComposer:    true,
		HasGit:         true,
	}

	bp := profiles.GetBlueprintForProfile(models.ProfileMinimalPHP, 22)
	plan, err := planner.GeneratePlan("srv-existing", inv, bp)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	// Nginx and PHP are already installed, so they should be marked as skip
	for _, item := range plan.Items {
		if item.Target == "nginx" && !item.Skip {
			t.Errorf("expected nginx to be skipped (already installed)")
		}
		if item.Target == "composer" && !item.Skip {
			t.Errorf("expected composer to be skipped (already installed)")
		}
	}
}

func TestPlannerSSHPortPreservation(t *testing.T) {
	inv := &inventory.ServerInventory{
		Distro:     "rocky",
		Family:     "rhel",
		Version:    "9.4",
		RAMTotalMB: 4096,
		DiskFreeGB: 50,
		SSHPort:    22022, // Custom non-standard SSH port
	}

	bp := profiles.GetBlueprintForProfile(models.ProfileFullPHP, 22022)
	plan, err := planner.GeneratePlan("srv-rhel", inv, bp)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.SSHPortPreserved != 22022 {
		t.Errorf("got preserved port %d, want 22022", plan.SSHPortPreserved)
	}

	foundSSHItem := false
	for _, item := range plan.Items {
		if item.Category == "firewall" && item.Target == "SSH port 22022" {
			foundSSHItem = true
			break
		}
	}
	if !foundSSHItem {
		t.Errorf("expected firewall item explicitly allowing SSH port 22022")
	}
}
