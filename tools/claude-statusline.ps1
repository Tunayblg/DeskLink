# Claude Code status line for DeskLink: saves the plan limits (5-hour and weekly
# usage in percent) so the phone can show them, and prints a short line.
#
# Several Claude Code sessions can run at once, and an idle one repeats the
# numbers from its last reply. Usage only grows inside a window, so a window is
# only overwritten by a newer one (later reset time) or by a higher percentage.
$ErrorActionPreference = 'Stop'

function Merge-Window($new, $old) {
    if (-not $new) { return $old }
    if (-not $old) { return $new }
    if ($new.resets_at -gt $old.resets_at) { return $new }
    if ($new.resets_at -eq $old.resets_at -and $new.used_percentage -ge $old.used_percentage) { return $new }
    return $old
}

try {
    $raw = [Console]::In.ReadToEnd()
    $j = $raw | ConvertFrom-Json
    $limits = $j.rate_limits
    $parts = @()
    if ($limits) {
        $dir = Join-Path $env:USERPROFILE '.claude'
        if ($env:CLAUDE_CONFIG_DIR) { $dir = $env:CLAUDE_CONFIG_DIR }
        $file = Join-Path $dir 'desklink-rate-limits.json'
        $old = $null
        if (Test-Path -LiteralPath $file) {
            try { $old = Get-Content -Raw -LiteralPath $file | ConvertFrom-Json } catch { $old = $null }
        }
        $merged = [ordered]@{}
        $five = Merge-Window $limits.five_hour $old.five_hour
        $seven = Merge-Window $limits.seven_day $old.seven_day
        if ($five) { $merged['five_hour'] = $five }
        if ($seven) { $merged['seven_day'] = $seven }
        $tmp = "$file.$PID.tmp"
        $json = $merged | ConvertTo-Json -Depth 5 -Compress
        [IO.File]::WriteAllText($tmp, $json, (New-Object Text.UTF8Encoding $false))
        Move-Item -Force -LiteralPath $tmp -Destination $file
        # The line shown in this session is its own data, not the merged one.
        if ($limits.five_hour) { $parts += ('5h {0:N0}%' -f $limits.five_hour.used_percentage) }
        if ($limits.seven_day) { $parts += ('7d {0:N0}%' -f $limits.seven_day.used_percentage) }
    }
    $model = $j.model.display_name
    if ($model) { $parts = , $model + $parts }
    [Console]::Out.Write(($parts -join ' | '))
} catch {
    [Console]::Out.Write('')
}
