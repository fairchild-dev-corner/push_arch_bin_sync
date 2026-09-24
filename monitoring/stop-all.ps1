# Force-stops the daemon, Prometheus, and Grafana by process name.
#
# Why this exists: on Windows, `make run-all`'s Ctrl+C cleanup goes through
# Git-Bash/MSYS's trap+kill mechanism, which was tested and confirmed NOT
# to reliably reach native .exe child processes (a Git-Bash/MSYS signal
# emulation limitation, not something fixable from the Makefile side). On
# Linux/macOS, real POSIX signal handling makes Ctrl+C alone sufficient and
# this script isn't needed.
#
# Safe to run any time - does nothing if a process isn't running.

$names = @("push_arch_bin_sync", "prometheus", "grafana-server", "grafana", "loki")
foreach ($name in $names) {
    $procs = Get-Process -Name $name -ErrorAction SilentlyContinue
    if ($procs) {
        $procs | Stop-Process -Force
        Write-Host "Stopped $name"
    } else {
        Write-Host "$name was not running"
    }
}
