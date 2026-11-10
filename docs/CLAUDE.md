# Claude usage on the phone

The home screen's CPU and GPU card has a second page: swipe it sideways. It shows how much of your Claude plan is used, as two rings (5 hours and 7 days) with the time left until each one resets.

## Without any setup

DeskLink reads the token counts Claude Code writes to `~/.claude/projects` and shows the five-hour window, today and the last seven days. Only numbers leave the PC, never what was said.

## Plan limits in percent

The percentages are not stored on the PC. Claude Code hands them to its status line command (Pro, Max and Ultra plans, after the first reply of a session). The script in [`tools/claude-statusline.ps1`](../tools/claude-statusline.ps1) saves them to `~/.claude/desklink-rate-limits.json`, which DeskLink reads.

1. Copy `tools/claude-statusline.ps1` to `%USERPROFILE%\.claude\desklink-statusline.ps1`.
2. Add this to `%USERPROFILE%\.claude\settings.json`:

```json
"statusLine": {
  "type": "command",
  "command": "powershell -NoProfile -ExecutionPolicy Bypass -File \"C:/Users/<you>/.claude/desklink-statusline.ps1\"",
  "refreshInterval": 60
}
```

Things to know:

- The numbers update only while Claude Code is open in a terminal, and are as fresh as that session's last reply. The VS Code extension does not run the status line, so use in it is not counted.
- With several sessions open, a window is only overwritten by a newer one or a higher percentage, so an idle session cannot set the numbers back.
- Without the script the page shows the token counts instead.

## Developer mode

The Developer tab lets you write or dictate a prompt on the phone and have it typed into the window in front on the PC. It works with Claude Code, Codex or any other tool in a terminal or editor, and the PC needs no microphone.

- **Dictate.** The round mic button uses Safari's speech recognition, and the keyboard's own mic key does the same job. Both need Dictation switched on in the iPhone's settings: Settings › General › Keyboard › Enable Dictation. If the button fails, the screen says so. Settings picks the language and can send right after you stop speaking.
- **Send to.** The pill at the top picks the program: the window in front, or one of the open developer windows: Claude, Codex and ChatGPT (also as a browser tab), terminals (Windows Terminal, PowerShell, cmd, Git Bash...) and code editors (Cursor, VS Code, JetBrains...). Other programs are not listed and are refused. DeskLink brings it to the front first and only types if it really is in front. Open the input field in that program yourself; Send then presses Enter there.
- **Live typing** (on by default). The text appears in the chosen program while you speak or write, a moment after each change, so you can watch it in WhatsApp or the terminal. When the speech recognizer rewrites its last words, DeskLink presses Backspace for the changed part and types the new one. If another window comes to the front, live typing stops and deletes nothing. **Send** then only presses Enter. Switch it off in Settings to type everything at once on Send.
- Without live typing, **Send** types the text and presses Enter, and **Type only** leaves it unsent. A line break in the text is typed as Shift+Enter.
- **Keys.** Esc, Tab, Shift+Tab, the up and down arrows, Ctrl+C and Enter.
- **Mini trackpad.** The pad on the left moves the PC's mouse like the Trackpad tab: slide to move, tap to click, two fingers to right-click or scroll, and the two strips at its bottom are the left and right buttons. It steps aside while the keyboard is up.
- **Claude Code extras.** The header shows plan usage (5 hours and 7 days in percent, or token counts until Claude Code has reported them). Next to it a status pill says which agent was used last (Claude Code or Codex CLI) and what it is doing: thinking, running a command, editing files, reading code, searching the web, or done. It looks at the end of the newest session log: for Claude Code in ~/.claude/projects (a prompt, tool call or tool result means working, a finished reply means done), for Codex in ~/.codex/sessions (between `task_started` and `task_complete`). A log that has been quiet for ten minutes counts as ready. Both can be switched off in Settings.
- With no program picked, the text goes to whichever window has focus on the PC.

The Settings tab at the bottom of the tab bar shows or hides every tab (Watch mode, Developer and the rest) and reorders them. It is stored on the phone.
