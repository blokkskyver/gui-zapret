package flowseal

import (
	"os"
	"testing"
)

func TestLocalServiceStatus(t *testing.T) {
	if os.Getenv("FLOWSEAL_TEST_ROOT") == "" {
		t.Skip("local read-only integration check")
	}
	status, err := ServiceStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.State == "" {
		t.Fatal("empty service state")
	}
	t.Logf("service state: %s", status.State)
}
