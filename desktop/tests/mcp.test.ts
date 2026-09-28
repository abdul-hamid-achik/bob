import { chmodSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeAll, describe, expect, it } from 'vitest'
import { McpClient } from '../src/main/mcp'

/**
 * A stand-in for `bob mcp serve`: newline-delimited JSON-RPC over stdio with
 * the same initialize / tools/list / tools/call shapes Bob emits.
 */
const FAKE_SERVER = `#!/usr/bin/env node
const tools = [
  { name: 'bob_context', description: 'bounded workspace contract', inputSchema: { type: 'object', properties: { workspace: { type: 'string' } }, additionalProperties: false } },
  { name: 'bob_plan', description: 'bounded plan and digest', inputSchema: { type: 'object', properties: { workspace: { type: 'string' } }, additionalProperties: false } }
];
let buffer = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buffer += chunk;
  let index = buffer.indexOf('\\n');
  while (index !== -1) {
    const line = buffer.slice(0, index).trim();
    buffer = buffer.slice(index + 1);
    index = buffer.indexOf('\\n');
    if (line) handle(line);
  }
});
function handle(line) {
  let message;
  try { message = JSON.parse(line); } catch { return; }
  if (message.method === 'initialize') {
    reply(message.id, { capabilities: { tools: { listChanged: true } }, instructions: 'Bob is a deterministic repository contract compiler.' });
  } else if (message.method === 'tools/list') {
    reply(message.id, { tools });
  } else if (message.method === 'tools/call') {
    if (message.params.name === 'boom') {
      reply(message.id, { content: [{ type: 'text', text: 'refused' }], isError: true });
      return;
    }
    reply(message.id, { content: [{ type: 'text', text: JSON.stringify({ ok: true, tool: message.params.name, args: message.params.arguments }) }], isError: false });
  }
  process.stderr.write('diagnostic line\\n');
}
function reply(id, result) {
  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id, result }) + '\\n');
}
process.stdin.on('end', () => process.exit(0));
`

let dir = ''
let serverPath = ''
let workspace = ''
let client: McpClient | null = null
const logs: string[] = []

beforeAll(() => {
  dir = mkdtempSync(join(tmpdir(), 'bob-console-mcp-'))
  serverPath = join(dir, 'fake-mcp-server')
  workspace = join(dir, 'workspace')
  mkdirSync(workspace, { recursive: true })
  writeFileSync(serverPath, FAKE_SERVER, 'utf8')
  chmodSync(serverPath, 0o755)
  writeFileSync(join(workspace, 'bob.yaml'), 'schema_version: 1\n', 'utf8')
})

afterEach(() => {
  client?.stop()
  client = null
})

describe('McpClient', () => {
  it('initializes, lists tools, and keeps the server instructions', async () => {
    client = new McpClient(serverPath, { onLog: (line) => logs.push(line) })
    const info = await client.start({ workspace })
    expect(info.started).toBe(true)
    expect(info.tools.map((tool) => tool.name)).toEqual(['bob_context', 'bob_plan'])
    expect(info.instructions).toContain('deterministic repository contract compiler')
    expect(info.workspace).toBe(workspace)
  })

  it('calls a tool and returns its content', async () => {
    client = new McpClient(serverPath)
    await client.start({ workspace })
    const result = await client.callTool('bob_plan', { workspace })
    expect(result.ok).toBe(true)
    expect(result.isError).toBe(false)
    const payload = JSON.parse(result.content[0]?.text ?? '{}') as { tool: string }
    expect(payload.tool).toBe('bob_plan')
    expect(result.durationMs).toBeGreaterThanOrEqual(0)
    expect(result.rawResponse).toContain('bob_plan')
  })

  it('surfaces a tool error without throwing', async () => {
    client = new McpClient(serverPath)
    await client.start({ workspace })
    const result = await client.callTool('boom', {})
    expect(result.ok).toBe(false)
    expect(result.isError).toBe(true)
    expect(result.content[0]?.text).toBe('refused')
  })

  it('refuses to call tools before the server starts', async () => {
    client = new McpClient(serverPath)
    const result = await client.callTool('bob_plan', {})
    expect(result.ok).toBe(false)
    expect(result.error).toMatch(/not running/)
  })

  it('collects stderr diagnostics separately from the protocol', async () => {
    logs.length = 0
    client = new McpClient(serverPath, { onLog: (line, stream) => logs.push(`${stream}:${line}`) })
    await client.start({ workspace })
    await client.callTool('bob_context', {})
    await new Promise((resolve) => setTimeout(resolve, 150))
    expect(logs.some((line) => line.startsWith('stderr:diagnostic line'))).toBe(true)
    expect(logs.some((line) => line.startsWith('stdout:'))).toBe(false)
  })

  it('rejects a missing binary or workspace', async () => {
    client = new McpClient(join(dir, 'missing'))
    await expect(client.start({ workspace })).rejects.toThrow(/no bob binary/)

    const started = new McpClient(serverPath)
    await expect(started.start({ workspace: join(dir, 'nope') })).rejects.toThrow(/does not exist/)
    started.stop()
  })

  it('passes the allowlist flags through to argv', async () => {
    client = new McpClient(serverPath)
    await client.start({ workspace, allowWorkspaces: [workspace], allowAnyWorkspace: true })
    expect(client.argv).toEqual([serverPath, 'mcp', 'serve', '--workspace', workspace, '--allow-workspace', workspace, '--allow-any-workspace'])
  })

  it('stops cleanly and reports the stopped state', async () => {
    client = new McpClient(serverPath)
    await client.start({ workspace })
    client.stop()
    expect(client.status()?.started).toBe(false)
  })
})

describe('McpClient teardown', () => {
  it('cleans up the temporary directory', () => {
    rmSync(dir, { recursive: true, force: true })
    expect(true).toBe(true)
  })
})
