package flowseal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Game struct {
	Mode string `json:"mode"`
	TCP  string `json:"tcp"`
	UDP  string `json:"udp"`
}

func DefaultGame() Game { return Game{"disabled", "1024-65535", "1024-65535"} }
func ValidatePorts(s string) error {
	for _, item := range strings.Split(s, ",") {
		ends := strings.Split(item, "-")
		if len(ends) > 2 {
			return fmt.Errorf("неверный диапазон: %s", item)
		}
		last := 0
		for _, e := range ends {
			n, err := strconv.Atoi(e)
			if err != nil || n < 1 || n > 65535 || n < last || strconv.Itoa(n) != e {
				return fmt.Errorf("неверный порт: %s", item)
			}
			last = n
		}
	}
	return nil
}
func (g Game) Validate() error {
	if g.Mode != "disabled" && g.Mode != "all" && g.Mode != "tcp" && g.Mode != "udp" {
		return fmt.Errorf("неизвестный режим Game Filter")
	}
	if err := ValidatePorts(g.TCP); err != nil {
		return err
	}
	return ValidatePorts(g.UDP)
}
func ReadGame(root string) (Game, error) {
	g := DefaultGame()
	b, err := os.ReadFile(filepath.Join(root, "utils", "game_filter.enabled"))
	if os.IsNotExist(err) {
		return g, nil
	}
	if err != nil {
		return g, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		p := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(p) == 1 {
			if p[0] == "all" || p[0] == "tcp" || p[0] == "udp" {
				g.Mode = p[0]
			}
			continue
		}
		switch p[0] {
		case "mode":
			g.Mode = p[1]
		case "tcp":
			g.TCP = p[1]
		case "udp":
			g.UDP = p[1]
		}
	}
	return g, g.Validate()
}
func ValidateRoot(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("нужен абсолютный путь к сборке Flowseal")
	}
	for _, f := range []string{"service.bat", "bin/winws.exe", "lists", "utils"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			return err
		}
	}
	return nil
}
func Strategies(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "general") && strings.EqualFold(filepath.Ext(e.Name()), ".bat") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

var variable = regexp.MustCompile(`%[^%]+%`)

func Parse(root, name string, g Game) ([]string, error) {
	if filepath.Base(name) != name || !strings.HasPrefix(strings.ToLower(name), "general") || !strings.EqualFold(filepath.Ext(name), ".bat") {
		return nil, fmt.Errorf("недопустимая стратегия")
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return nil, err
	}
	return ParseText(root, string(b), g)
}

// ParseText extracts only the winws start command; batch code is never executed.
func ParseText(root, src string, g Game) ([]string, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "^\n", " ")
	var command string
	for _, line := range strings.Split(src, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "start ") {
			i := strings.Index(lower, `"%bin%winws.exe"`)
			if i >= 0 {
				if command != "" {
					return nil, fmt.Errorf("несколько команд winws")
				}
				command = strings.TrimSpace(line)[i+len(`"%bin%winws.exe"`):]
			}
		}
	}
	if command == "" {
		return nil, fmt.Errorf("команда winws не найдена")
	}
	tcp, udp := "12", "12"
	if g.Mode == "all" || g.Mode == "tcp" {
		tcp = g.TCP
	}
	if g.Mode == "all" || g.Mode == "udp" {
		udp = g.UDP
	}
	legacy := tcp
	if g.Mode == "udp" {
		legacy = udp
	}
	values := map[string]string{"%bin%": filepath.Join(root, "bin") + string(os.PathSeparator), "%lists%": filepath.Join(root, "lists") + string(os.PathSeparator), "%gamefiltertcp%": tcp, "%gamefilterudp%": udp, "%gamefilter%": legacy}
	var unresolved string
	command = variable.ReplaceAllStringFunc(command, func(v string) string {
		if s, ok := values[strings.ToLower(v)]; ok {
			return s
		}
		unresolved = v
		return v
	})
	if unresolved != "" {
		return nil, fmt.Errorf("неподдерживаемая переменная %s", unresolved)
	}
	command = strings.ReplaceAll(command, "^!", "!")
	var args []string
	var token strings.Builder
	quoted := false
	for _, r := range command {
		switch {
		case r == '"':
			quoted = !quoted
		case (r == ' ' || r == '\t') && !quoted:
			if token.Len() > 0 {
				args = append(args, token.String())
				token.Reset()
			}
		case !quoted && strings.ContainsRune("&|<>^", r):
			return nil, fmt.Errorf("неподдерживаемый оператор batch")
		default:
			token.WriteRune(r)
		}
	}
	if quoted {
		return nil, fmt.Errorf("незакрытая кавычка")
	}
	if token.Len() > 0 {
		args = append(args, token.String())
	}
	if len(args) == 0 || !strings.HasPrefix(args[0], "--wf-") {
		return nil, fmt.Errorf("нет параметров фильтра")
	}
	for _, arg := range args {
		if arg == "--service" || strings.HasPrefix(arg, "--service=") {
			return nil, fmt.Errorf("параметр service задаётся приложением")
		}
	}
	return args, nil
}
