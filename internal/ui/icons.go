package ui

// icons é o conjunto de símbolos usado na interface. O conjunto "nerd" usa
// glifos da Nerd Font (faixa Font Awesome); "unicode" e "ascii" servem para
// terminais sem essa fonte.
type icons struct {
	Cursor  string
	Main    string
	Side    string
	Todo    string
	Done    string
	Tracked string

	StepDone string
	StepNow  string
	StepNext string

	Journal string
	Pending string
	Clock   string
	Due     string
	Late    string
	Steps   string
	Notes   string
	Trophy  string
	Spark   string
	Filter  string
	Theme   string

	Priority string
	Focus    string
	Lock     string

	BarFull  string
	BarEmpty string
	Rule     string
}

var iconSets = map[string]icons{
	"nerd": {
		Cursor:  "", // caret-right
		Main:    "", // shield
		Side:    "", // compass
		Todo:    "", // circle-o
		Done:    "", // check-circle
		Tracked: "", // map-marker

		StepDone: "", // check
		StepNow:  "", // dot-circle-o
		StepNext: "", // circle-o

		Journal: "", // book
		Pending: "", // flag
		Clock:   "", // clock-o
		Due:     "", // calendar
		Late:    "", // exclamation-circle
		Steps:   "", // tasks
		Notes:   "", // pencil
		Trophy:  "", // trophy
		Spark:   "", // star
		Filter:  "", // search
		Theme:   "", // paint-brush

		Priority: "", // fire
		Focus:    "", // hourglass
		Lock:     "", // lock

		BarFull: "━", BarEmpty: "─", Rule: "─",
	},
	"unicode": {
		Cursor: "▸", Main: "◆", Side: "◇", Todo: "○", Done: "●", Tracked: "✦",
		StepDone: "✓", StepNow: "◉", StepNext: "○",
		Journal: "◆", Pending: "⚑", Clock: "◷", Due: "◷", Late: "!", Steps: "≡",
		Notes: "¶", Trophy: "★", Spark: "✦", Filter: "/", Theme: "◐",
		Priority: "▲", Focus: "◔", Lock: "☆",
		BarFull: "━", BarEmpty: "─", Rule: "─",
	},
	"ascii": {
		Cursor: ">", Main: "#", Side: "~", Todo: "o", Done: "x", Tracked: "@",
		StepDone: "x", StepNow: ">", StepNext: ".",
		Journal: "#", Pending: "!", Clock: "@", Due: "@", Late: "!", Steps: "=",
		Notes: "%", Trophy: "*", Spark: "*", Filter: "/", Theme: "%",
		Priority: "^", Focus: "@", Lock: "-",
		BarFull: "=", BarEmpty: "-", Rule: "-",
	},
}

var iconOrder = []string{"nerd", "unicode", "ascii"}

func iconSet(name string) icons {
	if set, ok := iconSets[name]; ok {
		return set
	}
	return iconSets["nerd"]
}

// nextIconSet devolve o nome do conjunto seguinte, em ciclo.
func nextIconSet(name string) string {
	for i, n := range iconOrder {
		if n == name {
			return iconOrder[(i+1)%len(iconOrder)]
		}
	}
	return iconOrder[0]
}
