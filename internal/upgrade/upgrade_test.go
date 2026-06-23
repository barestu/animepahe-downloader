package upgrade

import "testing"

func TestSameVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"equal plain", "1.2.3", "1.2.3", true},
		{"equal with v on latest", "1.2.3", "v1.2.3", true},
		{"equal with v on current", "v1.2.3", "1.2.3", true},
		{"equal with v on both", "v1.2.3", "v1.2.3", true},
		{"equal with surrounding space", " v1.2.3 ", "1.2.3", true},
		{"newer differs", "1.2.3", "v1.3.0", false},
		{"older differs", "v2.0.0", "1.9.9", false},
		{"dev vs release", "dev", "v1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameVersion(tt.current, tt.latest); got != tt.want {
				t.Errorf("SameVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{" v1.2.3 ", "1.2.3"},
		{"vvv", "vv"}, // only one leading v stripped
		{"dev", "dev"},
	}
	for _, tt := range tests {
		if got := normalize(tt.in); got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
