# The light themes, on a light terminal. Same look as settings.tape, but with a
# light background, so it sets everything itself instead of sourcing it.
Output docs/images/themes-light.gif
Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 15
Set LineHeight 1.15
Set Width 1280
Set Height 760
Set Padding 18
Set Margin 26
Set MarginFill "#ddd6c6"
Set WindowBar Colorful
Set WindowBarSize 34
Set BorderRadius 12
Set Framerate 24
Set TypingSpeed 55ms
Set Theme { "name": "QuestTUI Light", "black": "#24301f", "red": "#c62828", "green": "#2e7d32", "yellow": "#a46a00", "blue": "#1f62b5", "magenta": "#7b3fa0", "cyan": "#2c6e6a", "white": "#d8d2c4", "brightBlack": "#6f7a68", "brightRed": "#e53935", "brightGreen": "#43a047", "brightYellow": "#c98a00", "brightBlue": "#3b7dd8", "brightMagenta": "#9c5cc4", "brightCyan": "#3b8f8a", "brightWhite": "#ffffff", "background": "#fbf8f1", "foreground": "#24301f", "selection": "#e2dccd", "cursor": "#a46a00" }
Env XDG_CONFIG_HOME "/tmp/qt-vhs"
Env PS1 "$ "
Env COLORTERM "truecolor"

Hide
Type "/tmp/qt-seed /tmp/qt-vhs full alvorada >/dev/null; clear; ./quest" Enter
Sleep 4.4s
Show

Sleep 2s
Type "t"
Sleep 500ms
Down
Sleep 400ms
Enter
Sleep 1.8s
Type "t"
Sleep 500ms
Down
Sleep 400ms
Enter
Sleep 1.8s
Type "t"
Sleep 500ms
Up
Sleep 300ms
Up
Sleep 400ms
Enter
Sleep 1.6s
