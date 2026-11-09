package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"gui-zapret/internal/flowseal"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func (a *App) strategyPath(name string) (string, error) {
	names, e := flowseal.Strategies(a.root)
	if e != nil {
		return "", e
	}
	for _, n := range names {
		if n == name {
			return filepath.Join(a.root, n), nil
		}
	}
	return "", fmt.Errorf("стратегия не найдена")
}
func (a *App) ReadStrategy(name string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, e := a.strategyPath(name)
	if e != nil {
		return "", e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return "", e
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("редактор поддерживает только UTF-8")
	}
	return string(b), nil
}
func (a *App) SaveStrategy(name, original, content string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, e := a.strategyPath(name)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	if string(b) != original {
		return fmt.Errorf("файл изменился на диске; откройте редактор заново")
	}
	if len(content) > 1<<20 {
		return fmt.Errorf("слишком большой файл")
	}
	g, e := flowseal.ReadGame(a.root)
	if e != nil {
		return e
	}
	if _, e = flowseal.ParseText(a.root, content, g); e != nil {
		return e
	}
	backup := p + "." + time.Now().Format("20060102-150405.000000000") + ".bak"
	if e = os.WriteFile(backup, b, 0600); e != nil {
		return e
	}
	if e = os.WriteFile(p, []byte(content), 0600); e != nil {
		return e
	}
	a.record("Сохранена стратегия " + name + " (создана резервная копия)")
	return nil
}
func selectedTestScript(src, root, name string) (string, error) {
	markers := []string{"$rootDir = Split-Path $PSScriptRoot", "$batFiles = Get-ChildItem", "$mode = Read-ModeSelection"}
	for _, m := range markers {
		if !strings.Contains(src, m) {
			return "", fmt.Errorf("эта версия теста не поддерживает запуск одной стратегии")
		}
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	src = strings.Replace(src, markers[0], "$rootDir = "+quote(root), 1)
	start := strings.Index(src, markers[1])
	end := strings.Index(src[start:], "\n")
	if end < 0 {
		return "", fmt.Errorf("неверный тестовый скрипт")
	}
	src = src[:start] + "$batFiles = @(Get-Item -LiteralPath " + quote(filepath.Join(root, name)) + ")\r" + src[start+end:]
	src = strings.Replace(src, markers[2], "$mode = 'all'", 1)
	return src, nil
}
func (a *App) TestStrategy(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, e := a.strategyPath(name); e != nil {
		return e
	}
	st, e := flowseal.ServiceStatus()
	if e != nil {
		return e
	}
	if st.State != "not-installed" {
		return fmt.Errorf("штатный тест требует удалить службу zapret; удалите её кнопкой «Удалить», затем повторите тест")
	}
	src, e := os.ReadFile(filepath.Join(a.root, "utils", "test zapret.ps1"))
	if e != nil {
		return e
	}
	script, e := selectedTestScript(string(src), a.root, name)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp("", "gui-zapret-test-*.ps1")
	if e != nil {
		return e
	}
	_, e = f.WriteString("\ufeff" + strings.TrimPrefix(script, "\ufeff"))
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	e = shell("runas", filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), `-NoProfile -NoExit -ExecutionPolicy Bypass -File `+windows.EscapeArg(f.Name()), a.root, 1)
	if e == nil {
		a.record("Открыт тест стратегии: " + name)
	}
	return e
}
