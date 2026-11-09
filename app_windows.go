package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
	"gui-zapret/internal/flowseal"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	root   string
	logs   []string
	config string
}
type Snapshot struct {
	Root        string          `json:"root"`
	Strategies  []string        `json:"strategies"`
	Game        flowseal.Game   `json:"game"`
	Status      flowseal.Status `json:"status"`
	Admin       bool            `json:"admin"`
	Logs        []string        `json:"logs"`
	Error       string          `json:"error"`
	AutoUpdates bool            `json:"autoUpdates"`
}

func newApp() *App { return &App{logs: []string{}} }
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	home, _ := os.UserHomeDir()
	a.root = filepath.Join(home, "Desktop", "zapret")
	dir, err := os.UserConfigDir()
	if err == nil {
		a.config = filepath.Join(dir, "gui-zapret", "config.json")
		if b, e := os.ReadFile(a.config); e == nil {
			var c struct{ Root string }
			if json.Unmarshal(b, &c) == nil && c.Root != "" {
				a.root = c.Root
			}
		}
	}
	for i, arg := range os.Args {
		if arg == "--root" && i+1 < len(os.Args) {
			a.root = os.Args[i+1]
		}
	}
	a.record("GUI готов. Служба не изменялась.")
}
func (a *App) record(s string) {
	line := time.Now().Format("15:04:05") + "  " + s
	a.logs = append(a.logs, line)
	if len(a.logs) > 100 {
		a.logs = a.logs[len(a.logs)-100:]
	}
	if a.config != "" {
		dir := filepath.Dir(a.config)
		if os.MkdirAll(dir, 0700) == nil {
			f, e := os.OpenFile(filepath.Join(dir, "gui.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if e == nil {
				defer f.Close()
				fmt.Fprintln(f, time.Now().Format("2006-01-02"), line)
			}
		}
	}
}
func (a *App) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := Snapshot{Root: a.root, Admin: windows.GetCurrentProcessToken().IsElevated(), Game: flowseal.DefaultGame(), Strategies: []string{}, Logs: append([]string{}, a.logs...)}
	add := func(err error) {
		if err != nil {
			s.Error += err.Error() + "; "
		}
	}
	add(flowseal.ValidateRoot(a.root))
	var err error
	s.Strategies, err = flowseal.Strategies(a.root)
	add(err)
	s.Game, err = flowseal.ReadGame(a.root)
	add(err)
	s.Status, err = flowseal.ServiceStatus()
	add(err)
	_, err = os.Stat(filepath.Join(a.root, "utils", "check_updates.enabled"))
	s.AutoUpdates = err == nil
	return s
}
func (a *App) ChooseRoot() (string, error) {
	root, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Папка сборки Flowseal (service.bat)"})
	if err != nil || root == "" {
		return "", err
	}
	if err = flowseal.ValidateRoot(root); err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config != "" {
		b, _ := json.Marshal(struct{ Root string }{root})
		if err = os.MkdirAll(filepath.Dir(a.config), 0700); err != nil {
			return "", err
		}
		if err = os.WriteFile(a.config, b, 0600); err != nil {
			return "", err
		}
	}
	a.root = root
	a.record("Выбрана папка: " + root)
	return root, nil
}
func (a *App) Preview(name string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	g, e := flowseal.ReadGame(a.root)
	if e != nil {
		return nil, e
	}
	return flowseal.Parse(a.root, name, g)
}
func (a *App) Action(action, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if action != "install" && action != "start" && action != "stop" && action != "remove" {
		return fmt.Errorf("неизвестное действие")
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("перезапустите GUI от администратора")
	}
	g := flowseal.DefaultGame()
	var e error
	if action == "install" {
		g, e = flowseal.ReadGame(a.root)
		if e == nil {
			_, e = flowseal.Parse(a.root, name, g)
		}
	}
	if e == nil && action == "install" {
		e = a.ensureLists()
	}
	if e == nil {
		e = flowseal.ServiceAction(action, a.root, name, g)
	}
	if e != nil {
		a.record(action + ": ОШИБКА: " + e.Error())
	} else {
		a.record(action + ": выполнено")
	}
	return e
}
func (a *App) ensureLists() error {
	for name, content := range map[string]string{"ipset-exclude-user.txt": "203.0.113.113/32\r\n", "list-general-user.txt": "# Never leave this file empty\r\ndomain.example.abc\r\n", "list-exclude-user.txt": "domain.example.abc\r\n"} {
		f, e := os.OpenFile(filepath.Join(a.root, "lists", name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if os.IsExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		_, e = f.WriteString(content)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func (a *App) SaveSettings(g flowseal.Game, updates bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e := flowseal.ValidateRoot(a.root); e != nil {
		return e
	}
	if e := g.Validate(); e != nil {
		return e
	}
	file := filepath.Join(a.root, "utils", "game_filter.enabled")
	if e := os.WriteFile(file, []byte(fmt.Sprintf("mode=%s\r\ntcp=%s\r\nudp=%s\r\n", g.Mode, g.TCP, g.UDP)), 0600); e != nil {
		return e
	}
	file = filepath.Join(a.root, "utils", "check_updates.enabled")
	var e error
	if updates {
		e = os.WriteFile(file, []byte("enabled\r\n"), 0600)
	} else {
		e = os.Remove(file)
		if os.IsNotExist(e) {
			e = nil
		}
	}
	if e != nil {
		return e
	}
	a.record("Настройки сохранены. Нажмите «Применить стратегию» для обновления службы.")
	return nil
}
func shell(verb, file, args, dir string, show int32) error {
	v, e := windows.UTF16PtrFromString(verb)
	if e != nil {
		return e
	}
	f, e := windows.UTF16PtrFromString(file)
	if e != nil {
		return e
	}
	p, e := windows.UTF16PtrFromString(args)
	if e != nil {
		return e
	}
	d, e := windows.UTF16PtrFromString(dir)
	if e != nil {
		return e
	}
	return windows.ShellExecute(0, v, f, p, d, show)
}
func (a *App) Elevate() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if windows.GetCurrentProcessToken().IsElevated() {
		return nil
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	if e = shell("runas", exe, "--root "+windows.EscapeArg(a.root), filepath.Dir(exe), 1); e != nil {
		return e
	}
	runtime.Quit(a.ctx)
	return nil
}
func (a *App) OpenTool(tool string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e := flowseal.ValidateRoot(a.root); e != nil {
		return e
	}
	var e error
	switch tool {
	case "tests":
		e = shell("runas", filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), `-NoProfile -NoExit -ExecutionPolicy Bypass -File `+windows.EscapeArg(filepath.Join(a.root, "utils", "test zapret.ps1")), a.root, 1)
	case "diagnostics":
		if strings.ContainsAny(a.root, "\"%!\r\n") {
			return fmt.Errorf("путь содержит недопустимые для cmd символы")
		}
		var source []byte
		source, e = os.ReadFile(filepath.Join(a.root, "service.bat"))
		if e != nil {
			return e
		}
		if !strings.Contains(string(source), "\n:service_diagnostics") {
			return fmt.Errorf("в service.bat отсутствует диагностика")
		}
		// Run the original diagnostic routine with its helper labels and distribution path.
		body := strings.ReplaceAll(string(source), "%~dp0", strings.TrimRight(a.root, `\`)+`\`)
		body = strings.ReplaceAll(body, "goto menu", "exit /b")
		var runner *os.File
		runner, e = os.CreateTemp("", "gui-zapret-diagnostics-*.bat")
		if e != nil {
			return e
		}
		_, e = runner.WriteString("@echo off\r\nsetlocal EnableDelayedExpansion\r\ngoto service_diagnostics\r\n" + body)
		closeErr := runner.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		e = shell("runas", filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), `/d /s /c ""`+runner.Name()+`""`, a.root, 1)
	case "menu":
		if strings.ContainsAny(a.root, "\"%\r\n") {
			return fmt.Errorf("путь содержит недопустимые для cmd символы")
		}
		e = shell("runas", filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), `/d /s /k ""`+filepath.Join(a.root, "service.bat")+`" admin"`, a.root, 1)
	case "lists":
		e = shell("open", filepath.Join(a.root, "lists"), "", a.root, 1)
	case "logs":
		e = shell("open", filepath.Dir(a.config), "", a.root, 1)
	default:
		return fmt.Errorf("неизвестный инструмент")
	}
	if e == nil {
		a.record("Открыто: " + tool)
	}
	return e
}

func (a *App) OpenRepository() {
	runtime.BrowserOpenURL(a.ctx, "https://github.com/Flowseal/zapret-discord-youtube")
}
