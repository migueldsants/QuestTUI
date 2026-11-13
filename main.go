package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"questtui/internal/config"
	"questtui/internal/lang"
	"questtui/internal/remind"
	"questtui/internal/store"
	"questtui/internal/theme"
	"questtui/internal/ui"
)

func main() {
	// O idioma vem do config.toml e vale também para a ajuda dos comandos.
	if cfg, err := config.Load(); err == nil {
		lang.Set(cfg.Lang)
	}

	root := &cobra.Command{
		Use:   "quest",
		Short: lang.T("Diário de quests para as tarefas do dia a dia", "A quest journal for everyday tasks"),
		Args:  cobra.NoArgs,
		RunE:  runTUI,
	}
	root.Flags().Bool(flag("vigilia"), false, lang.T(
		"abre direto no modo vigília (relógio e pendências)",
		"opens straight into vigil mode (clock and pending quests)"))
	root.AddCommand(addCmd(), listCmd(), doneCmd(), langCmd(), remindCmd())
	// As opções aceitam o nome nos dois idiomas (--prazo ou --due), e a
	// ajuda mostra o do idioma atual.
	root.SetGlobalNormalizationFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(flag(name))
	})

	if err := fang.Execute(context.Background(), root); err != nil {
		os.Exit(1)
	}
}

// open carrega a configuração e abre o banco na pasta de dados.
func open() (*store.Store, *config.Config, string, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, nil, "", err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, "", fmt.Errorf("config.toml: %w", err)
	}
	s, err := store.Open(filepath.Join(dir, "quests.db"))
	if err != nil {
		return nil, nil, "", err
	}
	return s, cfg, dir, nil
}

func runTUI(cmd *cobra.Command, _ []string) error {
	s, cfg, dir, err := open()
	if err != nil {
		return err
	}
	defer s.Close()

	vigil, _ := cmd.Flags().GetBool(flag("vigilia"))
	model := ui.New(s, cfg, theme.Load(filepath.Join(dir, "themes")), vigil)
	defer model.Close()
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func addCmd() *cobra.Command {
	var isMain, priority bool
	var due, notes string
	cmd := &cobra.Command{
		Use:   lang.T("add <título>", "add <title>"),
		Short: lang.T("Cria uma quest sem abrir o diário", "Creates a quest without opening the journal"),
		Example: lang.T(
			"  quest add comprar pão --prazo hoje\n  quest add \"entregar relatório\" --principal --prazo \"sexta 14h\"",
			"  quest add buy bread --due today\n  quest add \"hand in the report\" --main --due \"friday 14h\""),
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, _, _, err := open()
			if err != nil {
				return err
			}
			defer s.Close()

			now := time.Now()
			parsed, err := store.ParseDue(due, now)
			if err != nil {
				return fmt.Errorf(lang.T("prazo inválido: %w", "invalid due date: %w"), err)
			}
			q := store.Quest{Title: strings.Join(args, " "), Notes: notes, Kind: store.Side, Due: parsed, Priority: priority}
			if isMain {
				q.Kind = store.Main
			}
			if err := s.AddQuest(&q); err != nil {
				return err
			}
			fmt.Printf(lang.T("Quest #%d criada: %s\n", "Quest #%d created: %s\n"), q.ID, q.Title)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isMain, flag("principal"), "p", false, lang.T("marca como quest principal", "marks it as a main quest"))
	cmd.Flags().BoolVarP(&priority, flag("prioridade"), "!", false, lang.T("prioridade alta", "high priority"))
	cmd.Flags().StringVarP(&due, flag("prazo"), "d", "", lang.T("prazo: hoje, amanhã 14h, sexta, +3d, 20/11 14:00", "due date: today, tomorrow 14h, friday, +3d, 20/11 14:00"))
	cmd.Flags().StringVarP(&notes, flag("nota"), "n", "", lang.T("notas da quest", "quest notes"))
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: lang.T("Lista as quests pendentes", "Lists the pending quests"),
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			s, _, _, err := open()
			if err != nil {
				return err
			}
			defer s.Close()

			quests, err := s.Quests()
			if err != nil {
				return err
			}
			count := 0
			for _, q := range quests {
				if q.Done {
					continue
				}
				count++
				line := fmt.Sprintf("#%-3d %s", q.ID, q.Title)
				if q.Priority {
					line += lang.T("  [prioridade]", "  [priority]")
				}
				if q.Kind == store.Main {
					line += lang.T("  [principal]", "  [main]")
				}
				if len(q.Steps) > 0 {
					line += fmt.Sprintf("  %d/%d", q.StepsDone(), len(q.Steps))
				}
				if q.Due != "" {
					line += lang.T("  prazo ", "  due ") + store.FormatDue(q.Due)
				}
				fmt.Println(line)
			}
			if count == 0 {
				fmt.Println(lang.T("Nenhuma quest pendente.", "No pending quests."))
			}
			return nil
		},
	}
}

func doneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: lang.T("Conclui uma quest pelo número", "Completes a quest by its number"),
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(strings.TrimPrefix(args[0], "#"), 10, 64)
			if err != nil {
				return fmt.Errorf(lang.T("número inválido: %s", "invalid number: %s"), args[0])
			}
			s, _, _, err := open()
			if err != nil {
				return err
			}
			defer s.Close()

			quests, err := s.Quests()
			if err != nil {
				return err
			}
			for _, q := range quests {
				if q.ID != id {
					continue
				}
				if err := s.Complete(id); err != nil {
					return err
				}
				fmt.Printf(lang.T("Quest concluída: %s\n", "Quest completed: %s\n"), q.Title)
				return nil
			}
			return fmt.Errorf(lang.T("não existe quest #%d", "there is no quest #%d"), id)
		},
	}
}

func langCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "lang <pt|en>",
		Short:     lang.T("Troca o idioma (português ou inglês)", "Changes the language (Portuguese or English)"),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"pt", "en"},
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config.toml: %w", err)
			}
			cfg.Lang = args[0]
			if err := cfg.Save(); err != nil {
				return err
			}
			lang.Set(cfg.Lang)
			fmt.Println(lang.T("Idioma: português", "Language: English"))
			return nil
		},
	}
}

func remindCmd() *cobra.Command {
	var install, uninstall, test bool
	cmd := &cobra.Command{
		Use:     lang.T("lembretes", "reminders"),
		Aliases: []string{lang.T("reminders", "lembretes")},
		Short:   lang.T("Envia as notificações de prazo pendentes", "Sends the pending due-date notifications"),
		Long: lang.T(
			"Envia uma notificação para cada quest com prazo hoje, com hora que chegou ou atrasada (uma vez cada).\n"+
				"Com --instalar, cria uma tarefa agendada no Windows que roda este comando a cada 15 minutos,\n"+
				"para os lembretes chegarem mesmo com o diário fechado.",
			"Sends a notification for each quest due today, whose time has come, or overdue (once each).\n"+
				"With --install, creates a Windows scheduled task that runs this command every 15 minutes,\n"+
				"so reminders arrive even with the journal closed."),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			switch {
			case test:
				return remind.Send(lang.T("Teste", "Test"), lang.T("As notificações do QuestTUI estão funcionando.", "QuestTUI notifications are working."))
			case install:
				exe, err := os.Executable()
				if err != nil {
					return err
				}
				if real, err := filepath.EvalSymlinks(exe); err == nil {
					exe = real
				}
				if err := remind.Install(exe); err != nil {
					return err
				}
				fmt.Printf(lang.T("Tarefa \"%s\" criada: lembretes a cada 15 minutos.\n", "Task \"%s\" created: reminders every 15 minutes.\n"), remind.TaskName)
				return nil
			case uninstall:
				if err := remind.Uninstall(); err != nil {
					return err
				}
				fmt.Printf(lang.T("Tarefa \"%s\" removida.\n", "Task \"%s\" removed.\n"), remind.TaskName)
				return nil
			}
			s, _, _, err := open()
			if err != nil {
				return err
			}
			defer s.Close()
			return remind.Run(s, time.Now())
		},
	}
	cmd.Flags().BoolVar(&install, flag("instalar"), false, lang.T("cria a tarefa agendada no Windows", "creates the Windows scheduled task"))
	cmd.Flags().BoolVar(&uninstall, flag("remover"), false, lang.T("remove a tarefa agendada", "removes the scheduled task"))
	cmd.Flags().BoolVar(&test, flag("testar"), false, lang.T("mostra uma notificação de teste", "shows a test notification"))
	return cmd
}

// flagNames liga o nome de cada opção em português ao nome em inglês.
var flagNames = [][2]string{
	{"vigilia", "vigil"},
	{"principal", "main"},
	{"prioridade", "priority"},
	{"prazo", "due"},
	{"nota", "note"},
	{"instalar", "install"},
	{"remover", "remove"},
	{"testar", "test"},
}

// flag devolve o nome da opção no idioma atual, aceitando o nome em
// qualquer um dos dois idiomas.
func flag(name string) string {
	for _, n := range flagNames {
		if name == n[0] || name == n[1] {
			return lang.T(n[0], n[1])
		}
	}
	return name
}
