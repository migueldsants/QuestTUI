# Still screenshots for the README. VHS only takes screenshots while recording
# is visible, so the throwaway GIF goes to /tmp. The terminal can lag behind,
# so every screen gets time to settle before and after its screenshot.
Output "/tmp/qt-screens.gif"
Set Height 920
Source docs/demo/settings.tape

Hide
Type "/tmp/qt-seed /tmp/qt-vhs full >/dev/null; clear; ./quest" Enter
Sleep 4.4s
Show

Sleep 2s
Screenshot docs/images/journal.png
Sleep 1.5s
Type "s"
Sleep 2.5s
Screenshot docs/images/achievements.png
Sleep 1.5s
Escape
Sleep 400ms
Type "e"
Sleep 2.5s
Screenshot docs/images/form.png
Sleep 1.5s
Escape
Sleep 400ms
Type "c"
Sleep 2.5s
Screenshot docs/images/completed.png
Sleep 1.5s
Type "c"
Sleep 300ms
Type "?"
Sleep 2.5s
Screenshot docs/images/help.png
Sleep 1.5s
Escape
Sleep 300ms
