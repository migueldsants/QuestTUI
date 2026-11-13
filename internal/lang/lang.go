// Package lang escolhe entre os textos em português e em inglês.
package lang

// Current é o idioma em uso: "pt" ou "en".
var Current = "pt"

// Set define o idioma; qualquer valor que não seja "en" vira "pt".
func Set(code string) {
	if code == "en" {
		Current = "en"
	} else {
		Current = "pt"
	}
}

// Toggle alterna entre português e inglês e devolve o novo idioma.
func Toggle() string {
	if Current == "en" {
		Current = "pt"
	} else {
		Current = "en"
	}
	return Current
}

// T devolve o texto no idioma atual.
func T(pt, en string) string {
	if Current == "en" {
		return en
	}
	return pt
}
