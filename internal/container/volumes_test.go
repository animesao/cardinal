//go:build linux

package container

import "testing"

func TestParseVolumeSpecOptions(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		readOnly    bool
		propagation string
		noCopy      bool
	}{
		{name: "read only", value: "/host:/data:ro", readOnly: true},
		{name: "read write", value: "named:/data:rw", readOnly: false},
		{name: "propagation", value: "/host:/data:ro,rshared", readOnly: true, propagation: "rshared"},
		{name: "no copy", value: "named:/data:nocopy", noCopy: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseVolumeSpec(tt.value)
			if err != nil {
				t.Fatalf("ParseVolumeSpec(%q): %v", tt.value, err)
			}
			if spec.ReadOnly != tt.readOnly {
				t.Fatalf("ReadOnly = %v, want %v", spec.ReadOnly, tt.readOnly)
			}
			if spec.Propagation != tt.propagation {
				t.Fatalf("Propagation = %q, want %q", spec.Propagation, tt.propagation)
			}
			if spec.NoCopy != tt.noCopy {
				t.Fatalf("NoCopy = %v, want %v", spec.NoCopy, tt.noCopy)
			}
		})
	}
}

func TestVolumeMountFromSpecPreservesOptions(t *testing.T) {
	mount, err := VolumeMountFromSpec("named:/data:ro,rslave,nocopy")
	if err != nil {
		t.Fatalf("VolumeMountFromSpec: %v", err)
	}
	if mount.Source != "named" || mount.Target != "/data" {
		t.Fatalf("mount paths = %q -> %q", mount.Source, mount.Target)
	}
	if !mount.ReadOnly || mount.Propagation != "rslave" || !mount.NoCopy {
		t.Fatalf("mount options were not preserved: %+v", mount)
	}
}
