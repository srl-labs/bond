package bond

import "testing"

func TestConvertXPathToJSPath(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"Simple path": {
			input:    "/interfaces/interface",
			expected: ".interfaces.interface",
		},
		"Path with hyphens": {
			input:    "/system-config/hostname",
			expected: ".system_config.hostname",
		},
		"Path with list node": {
			input:    "/interfaces/interface[name=eth0]",
			expected: ".interfaces.interface{.name==\"eth0\"}",
		},
		"Complex path": {
			input:    "/network-instances/network-instance[name=default]/protocols/protocol[name=BGP]/bgp",
			expected: ".network_instances.network_instance{.name==\"default\"}.protocols.protocol{.name==\"BGP\"}.bgp",
		},
		"Empty input": {
			input:    "",
			expected: "",
		},
		"Input with multiple list nodes": {
			input:    "/a/b[x=1]/c[y=2]/d[z=3]",
			expected: ".a.b{.x==\"1\"}.c{.y==\"2\"}.d{.z==\"3\"}",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := convertXPathToJSPath(tt.input)
			if result != tt.expected {
				t.Errorf("convertXPathToJSPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
