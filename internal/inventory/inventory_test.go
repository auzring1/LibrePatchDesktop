package inventory

import "testing"

func TestFriendlyName(t *testing.T) {
	cases := []struct{ name, ver, want string }{
		{"Notepad++ (64-bit x64)", "8.9.8.1", "Notepad++"},
		{"Microsoft Visual C++ 2008 Redistributable - x86 9.0.30729.6161", "9.0.30729.6161", "Microsoft Visual C++ 2008 Redistributable (x86)"},
		{"Microsoft Visual C++ v14 Redistributable (x64) - 14.51.36247", "14.51.36247.0", "Microsoft Visual C++ v14 Redistributable"},
		{"Microsoft .NET SDK 10.0.404 (x64)", "10.4.426.53019", "Microsoft .NET SDK"},
		{"PowerShell 7.6.10.0-x64", "7.6.10.0", "PowerShell"},
		{"7-Zip 23.01 (x64)", "23.01", "7-Zip"},
		{"Node.js", "26.12.0", "Node.js"},
		{"Git", "2.56.0.2", "Git"},
		{"1.2.3", "1.2.3", "1.2.3"},
	}
	for _, c := range cases {
		if got := FriendlyName(c.name, c.ver); got != c.want {
			t.Errorf("FriendlyName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
