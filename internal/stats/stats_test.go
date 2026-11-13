package stats

import (
	"testing"
	"time"

	"questtui/internal/store"
)

func TestCompute(t *testing.T) {
	now := time.Date(2026, 11, 12, 18, 0, 0, 0, time.Local) // quinta
	at := func(daysAgo, n int, kind store.Kind) []store.Completion {
		var out []store.Completion
		for i := 0; i < n; i++ {
			out = append(out, store.Completion{Kind: kind, At: now.AddDate(0, 0, -daysAgo).Add(-time.Duration(i) * time.Minute)})
		}
		return out
	}
	var cs []store.Completion
	cs = append(cs, at(20, 1, store.Side)...) // sequência antiga de 1 dia
	for d := 10; d >= 6; d-- {                // 5 dias seguidos
		cs = append(cs, at(d, 1, store.Side)...)
	}
	cs = append(cs, at(2, 1, store.Main)...)
	cs = append(cs, at(1, 2, store.Side)...)
	cs = append(cs, at(0, 5, store.Side)...) // hoje: dia heroico

	s := Compute(cs, now)
	checks := map[string][2]int{
		"total":           {s.Total, 14},
		"hoje":            {s.Today, 5},
		"semana":          {s.Week, 8}, // de segunda (9/11) a hoje
		"sequência":       {s.Streak, 3},
		"maior seq.":      {s.BestStreak, 5},
		"melhor dia":      {s.BestDay, 5},
		"principais":      {s.MainDone, 1},
		"dias no gráfico": {len(s.Days), 14},
		"semanas":         {len(s.Weeks), 8},
		"hoje no gráfico": {s.Days[13], 5},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %d, esperava %d", name, c[0], c[1])
		}
	}
	got := 0
	for _, m := range s.Medals {
		if m.Got {
			got++
		}
	}
	// primeiro passo, aventureiro, grande jornada, dia heroico, constância
	if got != 5 {
		t.Errorf("%d medalhas, esperava 5", got)
	}

	// Sem conclusão hoje, a sequência de ontem continua valendo.
	if s := Compute(at(1, 1, store.Side), now); s.Streak != 1 {
		t.Errorf("sequência com conclusão só ontem = %d", s.Streak)
	}
	if s := Compute(nil, now); s.Total != 0 || s.Streak != 0 {
		t.Error("histórico vazio deveria dar zero")
	}
}
