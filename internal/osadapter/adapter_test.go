package osadapter

import "testing"

var (
	_ OSAdapter = (*DebianAdapter)(nil)
	_ OSAdapter = (*RhelAdapter)(nil)
)

func TestNewAdapterForFamily(t *testing.T) {
	cases := []string{FamilyRhel, FamilyDebian, "", "suse"}
	for _, family := range cases {
		a := NewAdapterForFamily(family)
		if a == nil {
			t.Fatalf("NewAdapterForFamily(%q) returned nil", family)
		}
		info := a.GetOSInfo()
		if info == nil {
			t.Fatalf("NewAdapterForFamily(%q) has nil OSInfo", family)
		}
		if family == FamilyRhel {
			if _, ok := a.(*RhelAdapter); !ok {
				t.Errorf("NewAdapterForFamily(%q) = %T, want *RhelAdapter", family, a)
			}
		} else if _, ok := a.(*DebianAdapter); !ok {
			t.Errorf("NewAdapterForFamily(%q) = %T, want *DebianAdapter", family, a)
		}
	}
}
