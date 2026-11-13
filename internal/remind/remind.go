// Package remind manda lembretes de prazo como notificações nativas.
//
// No WSL e no Windows a notificação sai pelo PowerShell do Windows; num Linux
// comum, pelo notify-send. Cada lembrete é registrado no banco para não ser
// repetido, então o diário aberto e a tarefa agendada podem rodar juntos.
package remind

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"questtui/internal/lang"
	"questtui/internal/store"
)

type Reminder struct {
	Title, Body string
}

// Due escolhe os lembretes que ainda não foram enviados e já os marca como
// enviados: prazo hoje (uma vez por dia), prazo com hora que chegou (uma vez)
// e quest atrasada (uma vez por dia).
func Due(s *store.Store, now time.Time) ([]Reminder, error) {
	quests, err := s.Quests()
	if err != nil {
		return nil, err
	}
	today := store.DateOnly(now)
	todayMark := today.Format("2006-01-02")

	var out []Reminder
	for _, q := range quests {
		if q.Done {
			continue
		}
		t, hasTime, ok := q.DueTime()
		if !ok {
			continue
		}
		day := store.DateOnly(t)
		var kind, mark, title string
		switch {
		case day.Before(today):
			kind, mark, title = "late", todayMark, lang.T("Quest atrasada", "Quest overdue")
		case hasTime && !now.Before(t):
			kind, mark, title = "due", q.Due, lang.T("Chegou a hora", "It's time")
		case day.Equal(today):
			kind, mark, title = "today", todayMark, lang.T("Prazo hoje", "Due today")
		default:
			continue
		}
		first, err := s.MarkNotified(q.ID, kind, mark)
		if err != nil {
			return out, err
		}
		if !first {
			continue
		}
		body := q.Title
		if hasTime {
			body += " · " + t.Format("15:04")
		}
		out = append(out, Reminder{Title: title, Body: body})
	}
	return out, nil
}

// Run envia os lembretes pendentes.
func Run(s *store.Store, now time.Time) error {
	rs, err := Due(s, now)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if err := Send(r.Title, r.Body); err != nil {
			return err
		}
	}
	return nil
}

// Send mostra uma notificação nativa.
func Send(title, body string) error {
	if ps := PowerShell(); ps != "" {
		script := fmt.Sprintf(toastScript, psQuote("QuestTUI · "+title), psQuote(body))
		return exec.Command(ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-EncodedCommand", Encode(script)).Run()
	}
	if p, err := exec.LookPath("notify-send"); err == nil {
		return exec.Command(p, "QuestTUI · "+title, body).Run()
	}
	return errors.New(lang.T("nenhum jeito de mostrar notificações neste sistema", "no way to show notifications on this system"))
}

// O app "Windows PowerShell" já vem registrado no Windows, então a
// notificação sai em nome dele sem precisar instalar nada. O cenário
// "reminder" deixa a notificação na tela até ser fechada, em vez de sumir
// sozinha depois de alguns segundos.
const toastScript = `$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$title = [System.Security.SecurityElement]::Escape(%s)
$body = [System.Security.SecurityElement]::Escape(%s)
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$xml.LoadXml("<toast scenario='reminder'><visual><binding template='ToastGeneric'><text>$title</text><text>$body</text></binding></visual><actions><action content='OK' arguments='dismiss' activationType='system'/></actions><audio src='ms-winsoundevent:Notification.Reminder'/></toast>")
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$toast.Priority = [Windows.UI.Notifications.ToastNotificationPriority]::High
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($toast)
`

// psQuote faz uma string literal do PowerShell. Além do apóstrofo comum, o
// PowerShell também aceita as aspas tipográficas simples como delimitador,
// então todas são duplicadas.
func psQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch {
		case r == '\'' || r == '‘' || r == '’' || r == '‚' || r == '‛':
			b.WriteRune(r)
			b.WriteRune(r)
		case r < 0x20:
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// Encode prepara um script para o -EncodedCommand do PowerShell (UTF-16LE em
// base64), o que evita qualquer problema de aspas na linha de comando.
func Encode(script string) string {
	units := utf16.Encode([]rune(script))
	buf := make([]byte, 2*len(units))
	for i, u := range units {
		buf[2*i], buf[2*i+1] = byte(u), byte(u>>8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// InWSL diz se o programa roda dentro do WSL.
func InWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}

// PowerShell devolve o caminho do powershell.exe do Windows, ou "" fora do
// Windows e do WSL.
func PowerShell() string {
	if runtime.GOOS != "windows" && !InWSL() {
		return ""
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p
	}
	const wslPath = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
	if _, err := os.Stat(wslPath); err == nil {
		return wslPath
	}
	return ""
}

// TaskName é o nome da tarefa agendada no Windows.
const TaskName = "QuestTUI Lembretes"

// Install cria (ou recria) uma tarefa agendada no Windows que roda
// "quest lembretes" a cada 15 minutos, sem abrir janela.
func Install(exe string) error {
	schtasks, err := schtasksPath()
	if err != nil {
		return err
	}
	run := fmt.Sprintf(`"%s" lembretes`, exe)
	if runtime.GOOS != "windows" {
		distro := os.Getenv("WSL_DISTRO_NAME")
		if distro == "" {
			return errors.New(lang.T("não consegui descobrir a distribuição do WSL (WSL_DISTRO_NAME)", "could not find the WSL distribution (WSL_DISTRO_NAME)"))
		}
		run = fmt.Sprintf(`conhost.exe --headless wsl.exe -d %s -e "%s" lembretes`, distro, exe)
	}
	out, err := exec.Command(schtasks, "/Create", "/F", "/SC", "MINUTE", "/MO", "15", "/TN", TaskName, "/TR", run).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Uninstall apaga a tarefa agendada.
func Uninstall() error {
	schtasks, err := schtasksPath()
	if err != nil {
		return err
	}
	out, err := exec.Command(schtasks, "/Delete", "/F", "/TN", TaskName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func schtasksPath() (string, error) {
	if runtime.GOOS != "windows" && !InWSL() {
		return "", errors.New(lang.T("a tarefa agendada só existe no Windows ou no WSL", "scheduled tasks only exist on Windows or WSL"))
	}
	if p, err := exec.LookPath("schtasks.exe"); err == nil {
		return p, nil
	}
	const wslPath = "/mnt/c/Windows/System32/schtasks.exe"
	if _, err := os.Stat(wslPath); err == nil {
		return wslPath, nil
	}
	return "", errors.New(lang.T("schtasks.exe não encontrado", "schtasks.exe not found"))
}
