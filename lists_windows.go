package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type ListInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type ListDocument struct {
	Root    string `json:"root"`
	Content string `json:"content"`
}

var listDescriptions = map[string]string{
	"list-general.txt":       "Основной список доменов для обхода DPI. Поставляется с Flowseal и может заменяться при обновлении. Свои домены лучше добавлять в list-general-user.txt.",
	"list-general-user.txt":  "Ваши дополнительные домены для обхода DPI. По одному домену в строке, без https:// и путей. Используется вместе с основным списком.",
	"list-google.txt":        "Домены Google и YouTube, для которых стратегии используют отдельные правила обработки. Поставляется с Flowseal.",
	"list-exclude.txt":       "Домены, исключённые из обработки в правилах с этим списком. Поставляется с Flowseal. Для своих исключений используйте list-exclude-user.txt.",
	"list-exclude-user.txt":  "Ваши исключения по доменам: сайты, трафик которых не нужно обрабатывать. По одному домену в строке, без https:// и путей.",
	"ipset-all.txt":          "Основной список IP-адресов и подсетей для правил IPSet. Один IP или сеть CIDR в строке (например, 192.0.2.0/24). Пустой файл означает любые адреса; 203.0.113.113/32 используется Flowseal как заглушка отключения.",
	"ipset-exclude.txt":      "IP-адреса и подсети, исключённые из обработки в правилах IPSet. Поставляется с Flowseal. Свои исключения добавляйте в ipset-exclude-user.txt.",
	"ipset-exclude-user.txt": "Ваши исключения по IP-адресам и подсетям. Один IP или сеть CIDR в строке. Заглушка 203.0.113.113/32 — штатное значение Flowseal.",
}

func (a *App) Lists() ([]ListInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(a.root, "lists"))
	if e != nil {
		return nil, e
	}
	out := []ListInfo{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
			desc := listDescriptions[entry.Name()]
			if desc == "" {
				desc = "Дополнительный текстовый список сборки. Назначение определяется параметрами --hostlist, --hostlist-exclude, --ipset или --ipset-exclude в выбранной стратегии."
			}
			out = append(out, ListInfo{entry.Name(), desc + " Действует в стратегиях, которые ссылаются на этот файл. Строки с # — комментарии."})
		}
	}
	return out, nil
}
func (a *App) listPath(name string) (string, error) {
	if filepath.Base(name) != name || !strings.EqualFold(filepath.Ext(name), ".txt") {
		return "", fmt.Errorf("недопустимое имя списка")
	}
	p := filepath.Join(a.root, "lists", name)
	info, e := os.Lstat(p)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("список должен быть обычным файлом")
	}
	return p, nil
}
func (a *App) ReadList(name string) (ListDocument, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	doc := ListDocument{Root: a.root}
	p, e := a.listPath(name)
	if e != nil {
		return doc, e
	}
	info, e := os.Stat(p)
	if e != nil {
		return doc, e
	}
	if info.Size() > 8<<20 {
		return doc, fmt.Errorf("список превышает 8 МБ")
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return doc, e
	}
	if !utf8.Valid(b) {
		return doc, fmt.Errorf("редактор поддерживает только UTF-8")
	}
	doc.Content = string(b)
	return doc, nil
}
func (a *App) SaveList(root, name, original, content string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if root != a.root {
		return fmt.Errorf("папка сборки изменилась; откройте список заново")
	}
	p, e := a.listPath(name)
	if e != nil {
		return e
	}
	if len(content) > 8<<20 || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return fmt.Errorf("нужен текст UTF-8 до 8 МБ без нулевых символов")
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	if string(b) != original {
		return fmt.Errorf("список изменён на диске; откройте его заново")
	}
	backup := p + "." + time.Now().Format("20060102-150405.000000000") + ".bak"
	if e = os.WriteFile(backup, b, 0600); e != nil {
		return e
	}
	if e = os.WriteFile(p, []byte(content), 0600); e != nil {
		return e
	}
	a.record("Сохранён список " + name + " (создана резервная копия)")
	return nil
}
