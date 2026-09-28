#!/usr/bin/env node
/**
 * End-to-end smoke run: boots the real Electron app hidden, drives every Bob
 * surface through the renderer bridge, and prints the report.
 *
 * Usage: node scripts/smoke.mjs [workspace]
 */
import { spawn } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const projectRoot = resolve(here, '..')
const repoRoot = resolve(projectRoot, '..')

const workspace = resolve(
  process.argv[2] ?? process.env.BOB_CONSOLE_SMOKE_WORKSPACE ?? repoRoot
)
const reportPath = join('/tmp', `bob-console-smoke-${Date.now()}.json`)

if (!existsSync(join(projectRoot, 'out/main/index.js'))) {
  console.error('out/ is missing — run `npm run build` first')
  process.exit(2)
}

const electronCli = join(projectRoot, 'node_modules/electron/cli.js')
const child = spawn(process.execPath, [electronCli, projectRoot], {
  cwd: projectRoot,
  env: {
    ...process.env,
    BOB_CONSOLE_SMOKE: '1',
    BOB_CONSOLE_SMOKE_WORKSPACE: workspace,
    BOB_CONSOLE_SMOKE_REPORT: reportPath,
    ELECTRON_ENABLE_LOGGING: '0'
  },
  stdio: ['ignore', 'pipe', 'pipe']
})

let stdout = ''
let stderr = ''
child.stdout.on('data', (chunk) => {
  stdout += chunk.toString()
  process.stdout.write(chunk)
})
child.stderr.on('data', (chunk) => {
  stderr += chunk.toString()
})

const timeout = setTimeout(() => {
  console.error('smoke run timed out after 240s')
  child.kill('SIGKILL')
}, 240_000)

child.on('exit', (code) => {
  clearTimeout(timeout)
  if (!existsSync(reportPath)) {
    console.error(`\nno report written (exit ${code})`)
    if (stderr.trim()) console.error(stderr.trim().split('\n').slice(-20).join('\n'))
    process.exit(1)
  }
  const report = JSON.parse(readFileSync(reportPath, 'utf8'))
  console.log(`\n${'─'.repeat(72)}`)
  console.log(`workspace: ${report.workspace}`)
  console.log(`binary:    ${report.binary}`)
  console.log(`${'─'.repeat(72)}`)
  for (const check of report.checks) {
    console.log(`${check.ok ? 'PASS' : 'FAIL'}  ${check.name.padEnd(46, ' ')} ${check.detail.slice(0, 90)}`)
  }
  console.log(`${'─'.repeat(72)}`)
  console.log(`${report.passed} passed, ${report.failed} failed  →  ${reportPath}`)
  if (report.failed > 0) {
    console.error('\nstderr tail:\n' + stderr.trim().split('\n').slice(-15).join('\n'))
  }
  process.exit(report.failed === 0 ? 0 : 1)
})
