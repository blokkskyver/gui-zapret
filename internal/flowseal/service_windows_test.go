package flowseal

import (
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"os"
	"testing"
)

type fakeStopService struct {
	states     []svc.State
	controls   int
	controlErr error
}

func (f *fakeStopService) Query() (svc.Status, error) {
	if len(f.states) == 0 {
		return svc.Status{}, fmt.Errorf("unexpected query")
	}
	s := f.states[0]
	if len(f.states) > 1 {
		f.states = f.states[1:]
	}
	return svc.Status{State: s}, nil
}
func (f *fakeStopService) Control(svc.Cmd) (svc.Status, error) {
	f.controls++
	e := f.controlErr
	f.controlErr = nil
	return svc.Status{}, e
}
func TestStopPendingTransitions(t *testing.T) {
	for _, c := range []struct {
		name       string
		states     []svc.State
		controlErr error
		controls   int
	}{
		{"already-stopped", []svc.State{svc.Stopped}, nil, 0},
		{"starting", []svc.State{svc.StartPending, svc.Running, svc.Stopped}, nil, 1},
		{"already-stopping", []svc.State{svc.StopPending, svc.Stopped}, nil, 0},
		{"transient-control", []svc.State{svc.Running, svc.Running, svc.Stopped}, windows.ERROR_SERVICE_CANNOT_ACCEPT_CTRL, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeStopService{states: c.states, controlErr: c.controlErr}
			if e := stop(f); e != nil {
				t.Fatal(e)
			}
			if f.controls != c.controls {
				t.Fatalf("stop controls: %d", f.controls)
			}
		})
	}
}
func TestLocalServiceCycle(t *testing.T) {
	if os.Getenv("FLOWSEAL_SERVICE_CYCLE") != "1" {
		t.Skip("explicit opt-in: stop and restart the real service")
	}
	st, e := ServiceStatus()
	if e != nil {
		t.Fatal(e)
	}
	if st.State != "running" {
		t.Fatal("expected a running service to restore")
	}
	defer func() {
		if e := ServiceAction("start", "", "", DefaultGame()); e != nil {
			t.Errorf("RESTORE FAILED: %v", e)
		}
	}()
	if e := ServiceAction("stop", "", "", DefaultGame()); e != nil {
		t.Fatal(e)
	}
	st, e = ServiceStatus()
	if e != nil || st.State != "stopped" {
		t.Fatalf("stop verification: %s %v", st.State, e)
	}
	if e := ServiceAction("start", "", "", DefaultGame()); e != nil {
		t.Fatal(e)
	}
	t.Log("real service stopped and restored")
}

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
