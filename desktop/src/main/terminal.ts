import { execFile } from 'node:child_process'
import { platform } from 'node:os'
import { spawnCapture } from './runner'

export interface TerminalLaunch {
  launched: boolean
  command: string
  error: string | null
}

function quoteForShell(command: string): string {
  return command.replace(/(["\\$`])/g, '\\$1')
}

/**
 * Hands a real TUI (bob studio) to the operator's terminal.
 * The console never renders a TUI itself and never runs a shell from the renderer.
 */
export async function launchInTerminal(command: string): Promise<TerminalLaunch> {
  const os = platform()
  try {
    if (os === 'darwin') {
      const script = `tell application "Terminal" to do script "${quoteForShell(command)}"`
      await new Promise<void>((promiseResolve, promiseReject) => {
        execFile('osascript', ['-e', script], { timeout: 10_000 }, (error) =>
          error ? promiseReject(error) : promiseResolve()
        )
      })
      return { launched: true, command, error: null }
    }
    if (os === 'linux') {
      const shells = ['x-terminal-emulator', 'gnome-terminal', 'konsole', 'xfce4-terminal']
      for (const shell of shells) {
        const found = await new Promise<boolean>((promiseResolve) => {
          execFile('which', [shell], { timeout: 4000 }, (error) => promiseResolve(!error))
        })
        if (!found) continue
        const argv = shell === 'gnome-terminal' || shell === 'xfce4-terminal' ? ['--', 'bash', '-lc', command] : ['-e', command]
        execFile(shell, argv, { timeout: 10_000 }, () => undefined)
        return { launched: true, command, error: null }
      }
      return { launched: false, command, error: 'no known terminal emulator found on PATH' }
    }
    return { launched: false, command, error: `launching a terminal is not supported on ${os}` }
  } catch (error) {
    return { launched: false, command, error: error instanceof Error ? error.message : String(error) }
  }
}

export async function whichTool(command: string): Promise<string | null> {
  return new Promise((promiseResolve) => {
    execFile('which', [command], { timeout: 4000 }, (error, stdout) => {
      if (error) return promiseResolve(null)
      const first = stdout.split('\n').map((line) => line.trim()).filter(Boolean)[0]
      promiseResolve(first ?? null)
    })
  })
}

export async function runDetached(command: string, argv: string[], cwd: string): Promise<{ ok: boolean; error: string | null }> {
  const outcome = await spawnCapture({ command, argv, cwd, timeoutMs: 20_000 })
  return { ok: outcome.exitCode === 0, error: outcome.exitCode === 0 ? null : outcome.stderr.trim() || `exit ${outcome.exitCode}` }
}
