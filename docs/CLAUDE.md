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
