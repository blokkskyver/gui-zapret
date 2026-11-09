package flowseal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPorts(t *testing.T) {
	for _, s := range []string{"1", "65535", "443,1024-65535"} {
		if e := ValidatePorts(s); e != nil {
			t.Errorf("%s: %v", s, e)
		}
	}
	for _, s := range []string{"", "0", "65536", "2-1", "01", "1,", "1-2-3", "1&whoami", "-1"} {
		if ValidatePorts(s) == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func TestParse(t *testing.T) {
	root := filepath.Join(t.TempDir(), "path with spaces")
	src := "@echo off\r\nstart \"zapret\" /min \"%BIN%winws.exe\" --wf-tcp=443,%GameFilterTCP% ^\r\n--hostlist=\"%LISTS%list-general.txt\" --filter-udp=%GameFilterUDP%\r\n"
	args, e := ParseText(root, src, Game{"tcp", "1000-2000", "3000"})
	if e != nil {
		t.Fatal(e)
	}
	if len(args) != 3 || args[0] != "--wf-tcp=443,1000-2000" || args[1] != "--hostlist="+filepath.Join(root, "lists", "list-general.txt") || args[2] != "--filter-udp=12" {
		t.Fatalf("unexpected: %#v", args)
	}
	for _, suffix := range []string{" & calc.exe", " %UNKNOWN%", " \"bad", " --service"} {
		if _, e := ParseText(root, strings.TrimSpace(strings.ReplaceAll(src, "^\r\n", " "))+suffix, DefaultGame()); e == nil {
			t.Errorf("accepted %s", suffix)
		}
	}
}
func TestLocalStrategies(t *testing.T) {
	root := os.Getenv("FLOWSEAL_TEST_ROOT")
	if root == "" {
		t.Skip("set FLOWSEAL_TEST_ROOT to check an installed distribution")
	}
	names, e := Strategies(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(names) == 0 {
		t.Fatal("no strategies")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for _, mode := range []string{"disabled", "all", "tcp", "udp"} {
				g := DefaultGame()
				g.Mode = mode
				args, e := Parse(root, name, g)
				if e != nil {
					t.Fatal(e)
				}
				for _, arg := range args {
					parts := strings.SplitN(arg, "=", 2)
					if len(parts) == 2 && filepath.IsAbs(parts[1]) {
						if _, e = os.Stat(parts[1]); e != nil {
							t.Errorf("missing input %s", parts[1])
						}
					}
				}
			}
		})
	}
	t.Logf("validated %d strategies in four modes", len(names))
}
