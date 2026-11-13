//go:build ignore

// Seed builds the sample journal used to record the README images.
//
//	go run docs/demo/seed.go <config dir> [full|finale]
//
// It writes <config dir>/questtui/config.toml and quests.db, so point
// XDG_CONFIG_HOME at the same directory when running the app.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"questtui/internal/store"
)

const config = `theme = "heroi"
idle_minutes = 0
clock_24h = true
icons = "nerd"
lang = "en"
sound = false
reminders = false
focus_minutes = 25
break_minutes = 5
editor_mode = "padrao"
external_editor = ""
notes_preview = true
`

type quest struct {
	title    string
	main     bool
	priority bool
	tracked  bool
	due      string // anything store.ParseDue accepts
	steps    []string
	done     int // how many steps are already done
	notes    string
}

var full = []quest{
	{
		title: "Ship the portfolio site", main: true, priority: true, tracked: true, due: "friday 18:00",
		steps: []string{"Write the case studies", "Polish the landing page", "Record a demo video", "Deploy to production"},
		done:  2,
		notes: "## Launch plan\n\n- [x] Pick the domain\n- [ ] Set up **analytics**\n- [ ] Share it on `#show-and-tell`\n\n> Ship small, ship often.",
	},
	{
		title: "Learn Go generics", main: true, due: "+6d",
		steps: []string{"Read the official tutorial", "Build a typed cache", "Write a blog post"},
		done:  1,
	},
	{title: "Renew the passport", due: "late"},
	{title: "Call the dentist", due: "tomorrow 09:30"},
	{
		title: "Buy groceries", due: "today",
		steps: []string{"Coffee beans", "Oat milk", "Fresh bread"},
	},
	{title: "Water the plants"},
	{title: "Plan the weekend trip"},
}

var finale = []quest{
	{
		title: "Inbox zero", main: true, tracked: true, due: "today",
		steps: []string{"Archive the newsletters", "Answer the last email"},
		done:  1,
	},
}

var history = []string{
	"Fix the bike", "Read Dune", "Clean the desk", "Book a haircut", "Pay the internet bill",
	"Update the CV", "Back up the photos", "Reply to Ana", "Go for a run", "Refactor the auth module",
	"Return the library books", "Cook a new recipe", "Sort the receipts", "Stretch for 20 minutes",
	"Order new headphones", "Write the weekly review", "Call grandma", "Organize the Downloads folder",
	"Finish the online course", "Meal prep for the week", "Review the pull request", "Change the bed sheets",
	"Plan the sprint", "Practice guitar", "Take out the recycling", "Send the invoice",
	"Update the dependencies", "Walk 10,000 steps", "Clean the fridge", "Learn a new shortcut",
}

// Completions per day, from 13 days ago until today.
var perDay = []int{1, 2, 0, 3, 2, 1, 4, 2, 0, 1, 3, 2, 4, 2}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run docs/demo/seed.go <config dir> [full|finale]")
		os.Exit(2)
	}
	dir := filepath.Join(os.Args[1], "questtui")
	scenario := "full"
	if len(os.Args) > 2 {
		scenario = os.Args[2]
	}
	must(os.RemoveAll(dir))
	must(os.MkdirAll(dir, 0o755))
	must(os.WriteFile(filepath.Join(dir, "config.toml"), []byte(config), 0o644))

	dbPath := filepath.Join(dir, "quests.db")
	s, err := store.Open(dbPath)
	must(err)
	now := time.Now()

	// History first, so the finished quests sit below the active ones.
	type done struct {
		id   int64
		at   time.Time
		main bool
	}
	var finished []done
	k := 0
	for daysAgo := len(perDay) - 1; daysAgo >= 0; daysAgo-- {
		n := perDay[len(perDay)-1-daysAgo]
		if scenario == "finale" && daysAgo == 0 {
			n = 0 // the finale completes today's quest live
		}
		for i := 0; i < n; i++ {
			q := store.Quest{Title: history[k%len(history)], Kind: store.Side}
			if k == 5 {
				q.Kind = store.Main
			}
			k++
			must(s.AddQuest(&q))
			must(s.Complete(q.ID))
			at := store.DateOnly(now).AddDate(0, 0, -daysAgo).Add(time.Duration(9+3*i) * time.Hour)
			if daysAgo == 0 && at.After(now) {
				at = now.Add(-time.Duration(i+1) * 20 * time.Minute)
			}
			finished = append(finished, done{q.ID, at, q.Kind == store.Main})
		}
	}

	quests := full
	if scenario == "finale" {
		quests = finale
	}
	for _, d := range quests {
		q := store.Quest{Title: d.title, Kind: store.Side, Priority: d.priority, Notes: d.notes}
		if d.main {
			q.Kind = store.Main
		}
		switch d.due {
		case "":
		case "late":
			q.Due = now.AddDate(0, 0, -2).Format("2006-01-02")
		default:
			due, err := store.ParseDue(d.due, now)
			must(err)
			q.Due = due
		}
		must(s.AddQuest(&q))
		for _, step := range d.steps {
			must(s.AddStep(q.ID, step))
		}
		all, err := s.Quests()
		must(err)
		for _, x := range all {
			if x.ID != q.ID {
				continue
			}
			for i := 0; i < d.done; i++ {
				must(s.SetStepDone(x.Steps[i].ID, true))
			}
		}
		if d.tracked {
			must(s.ToggleTracked(q.ID))
		}
	}
	must(s.Close())

	// Spread the history over the last two weeks.
	db, err := sql.Open("sqlite", dbPath)
	must(err)
	defer db.Close()
	for _, f := range finished {
		stamp := f.at.Format(time.RFC3339)
		_, err := db.Exec(`UPDATE quests SET done_at = ? WHERE id = ?`, stamp, f.id)
		must(err)
		_, err = db.Exec(`UPDATE completions SET done_at = ? WHERE quest_id = ?`, stamp, f.id)
		must(err)
	}
	fmt.Printf("seeded %s (%s): %d active, %d completed\n", dir, scenario, len(quests), len(finished))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
