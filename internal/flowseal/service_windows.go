package flowseal

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"path/filepath"
	"strings"
	"time"
)

type Status struct {
	State    string `json:"state"`
	Strategy string `json:"strategy"`
	Command  string `json:"command"`
}

func ServiceStatus() (Status, error) {
	out := Status{State: "unknown"}
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return out, err
	}
	m := &mgr.Mgr{Handle: h}
	defer m.Disconnect()
	name, _ := windows.UTF16PtrFromString("zapret")
	sh, err := windows.OpenService(m.Handle, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		out.State = "not-installed"
		return out, nil
	}
	if err != nil {
		return out, err
	}
	s := &mgr.Service{Name: "zapret", Handle: sh}
	defer s.Close()
	state, err := s.Query()
	if err != nil {
		return out, err
	}
	out.State = map[svc.State]string{svc.Stopped: "stopped", svc.StartPending: "starting", svc.StopPending: "stopping", svc.Running: "running", svc.Paused: "paused", svc.PausePending: "pausing", svc.ContinuePending: "continuing"}[state.State]
	if out.State == "" {
		out.State = "unknown"
	}
	out.Command, err = serviceCommand()
	if err != nil {
		return out, err
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\zapret`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		out.Strategy, _, _ = key.GetStringValue("zapret-discord-youtube")
	}
	return out, nil
}

// QueryServiceConfig has an 8 KiB buffer limit; Flowseal command lines can exceed it.
func serviceCommand() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\zapret`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	command, _, err := key.GetStringValue("ImagePath")
	return command, err
}

type serviceController interface {
	Query() (svc.Status, error)
	Control(svc.Cmd) (svc.Status, error)
}

func waitState(s serviceController, desired svc.State) error {
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err != nil {
			return err
		}
		if status.State == desired {
			return nil
		}
		if desired == svc.Running && status.State == svc.Stopped {
			return fmt.Errorf("служба остановилась: Win32=%d, service=%d", status.Win32ExitCode, status.ServiceSpecificExitCode)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("служба не достигла нужного состояния за 25 секунд; обновите статус")
}
func stop(s serviceController) error {
	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State == svc.Stopped {
		return nil
	}
	deadline := time.Now().Add(25 * time.Second)
	for st.State != svc.StopPending {
		if st.State == svc.Stopped {
			return nil
		}
		if st.State != svc.StartPending {
			_, err = s.Control(svc.Stop)
			if err == nil {
				break
			}
			if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
				return waitState(s, svc.Stopped)
			}
			if !errors.Is(err, windows.ERROR_SERVICE_CANNOT_ACCEPT_CTRL) {
				return fmt.Errorf("остановка службы: %w", err)
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("служба не принимает команду остановки; состояние %d", st.State)
		}
		time.Sleep(200 * time.Millisecond)
		st, err = s.Query()
		if err != nil {
			return err
		}
	}
	return waitState(s, svc.Stopped)
}
func ServiceAction(action, root, name string, g Game) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("перезапустите GUI от администратора")
	}
	var args []string
	var err error
	if action == "install" {
		if err = ValidateRoot(root); err != nil {
			return err
		}
		args, err = Parse(root, name, g)
		if err != nil {
			return err
		}
	}
	access := uint32(windows.SC_MANAGER_CONNECT)
	if action == "install" {
		access |= windows.SC_MANAGER_CREATE_SERVICE
	}
	h, err := windows.OpenSCManager(nil, nil, access)
	if err != nil {
		return err
	}
	m := &mgr.Mgr{Handle: h}
	defer m.Disconnect()
	rights := uint32(windows.SERVICE_QUERY_STATUS)
	switch action {
	case "start":
		rights |= windows.SERVICE_START
	case "stop":
		rights |= windows.SERVICE_STOP
	case "remove":
		rights |= windows.SERVICE_STOP | windows.DELETE
	case "install":
		rights |= windows.SERVICE_STOP | windows.SERVICE_START | windows.SERVICE_CHANGE_CONFIG
	}
	serviceName, _ := windows.UTF16PtrFromString("zapret")
	sh, err := windows.OpenService(m.Handle, serviceName, rights)
	s := &mgr.Service{Name: "zapret", Handle: sh}
	if action == "install" && errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		s, err = m.CreateService("zapret", filepath.Join(root, "bin", "winws.exe"), mgr.Config{DisplayName: "zapret", Description: "Zapret DPI bypass software", StartType: mgr.StartAutomatic}, args...)
		if err != nil {
			return err
		}
		defer s.Close()
	} else {
		if err != nil {
			return err
		}
		defer s.Close()
		command, e := serviceCommand()
		if e != nil {
			return e
		}
		if !strings.Contains(strings.ToLower(command), "winws.exe") {
			return fmt.Errorf("служба zapret не использует winws.exe")
		}
		if action == "install" {
			if err = stop(s); err != nil {
				return err
			}
			parts := []string{windows.EscapeArg(filepath.Join(root, "bin", "winws.exe"))}
			for _, a := range args {
				parts = append(parts, windows.EscapeArg(a))
			}
			binaryPath, e := windows.UTF16PtrFromString(strings.Join(parts, " "))
			if e != nil {
				return e
			}
			if err = windows.ChangeServiceConfig(s.Handle, windows.SERVICE_NO_CHANGE, mgr.StartAutomatic, windows.SERVICE_NO_CHANGE, binaryPath, nil, nil, nil, nil, nil, nil); err != nil {
				return err
			}
		}
	}
	switch action {
	case "install":
		key, e := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\zapret`, registry.SET_VALUE)
		if e != nil {
			return e
		}
		e = key.SetStringValue("zapret-discord-youtube", name)
		key.Close()
		if e != nil {
			return e
		}
		if err = s.Start(); err != nil {
			return err
		}
		return waitState(s, svc.Running)
	case "start":
		st, e := s.Query()
		if e != nil {
			return e
		}
		if st.State == svc.Running {
			return nil
		}
		if err = s.Start(); err != nil {
			return err
		}
		return waitState(s, svc.Running)
	case "stop":
		return stop(s)
	case "remove":
		if err = stop(s); err != nil {
			return err
		}
		return s.Delete()
	default:
		return fmt.Errorf("неизвестное действие")
	}
}
