import { app, BrowserWindow, Menu, nativeImage, nativeTheme, shell } from 'electron'
import { existsSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { registerIpc } from './ipc'
import { createState, refreshBinary, type ConsoleState } from './state'
import { runSmoke } from './smoke'
import type { AppCommand } from '../shared/ipc'

const here = dirname(fileURLToPath(import.meta.url))

const isSmoke = process.env.BOB_CONSOLE_SMOKE === '1'

/** Source-checkout icon; packaged builds carry the icns inside the bundle. */
const devIconPath = join(here, '../../build/icon.png')

function windowIcon(): string | undefined {
  return existsSync(devIconPath) ? devIconPath : undefined
}

let mainWindow: BrowserWindow | null = null
let state: ConsoleState | null = null

function sendCommand(command: AppCommand): void {
  if (mainWindow && !mainWindow.isDestroyed()) mainWindow.webContents.send('app:command', command)
}

function buildMenu(): Menu {
  const template: Electron.MenuItemConstructorOptions[] = [
    {
      label: app.name,
      submenu: [
        { role: 'about' },
        { type: 'separator' },
        { role: 'hide' },
        { role: 'hideOthers' },
        { role: 'unhide' },
        { type: 'separator' },
        { role: 'quit' }
      ]
    },
    {
      label: 'Workspace',
      submenu: [
        {
          label: 'Open workspace…',
          accelerator: 'CmdOrCtrl+O',
          click: () => sendCommand('open-workspace')
        },
        { type: 'separator' },
        {
          label: 'Re-resolve Bob binary',
          accelerator: 'CmdOrCtrl+Shift+B',
          click: () => {
            if (state) void refreshBinary(state)
            sendCommand('refresh')
          }
        },
        {
          label: 'Refresh current panel',
          accelerator: 'CmdOrCtrl+R',
          click: () => sendCommand('refresh')
        }
      ]
    },
    {
      label: 'Bob',
      submenu: [
        { label: 'Run plan', accelerator: 'CmdOrCtrl+P', click: () => sendCommand('run-plan') },
        { label: 'Run check', accelerator: 'CmdOrCtrl+Shift+K', click: () => sendCommand('run-check') },
        { type: 'separator' },
        { label: 'Command palette', accelerator: 'CmdOrCtrl+K', click: () => sendCommand('command-palette') },
        { label: 'Console', accelerator: 'CmdOrCtrl+J', click: () => sendCommand('focus-console') },
        { type: 'separator' },
        {
          label: 'Toggle theme',
          accelerator: 'CmdOrCtrl+Shift+T',
          click: () => sendCommand('toggle-theme')
        }
      ]
    },
    {
      label: 'Edit',
      submenu: [
        { role: 'undo' },
        { role: 'redo' },
        { type: 'separator' },
        { role: 'cut' },
        { role: 'copy' },
        { role: 'paste' },
        { role: 'selectAll' }
      ]
    },
    {
      label: 'View',
      submenu: [
        { role: 'reload' },
        { role: 'forceReload' },
        { role: 'toggleDevTools' },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' }
      ]
    },
    {
      label: 'Window',
      submenu: [{ role: 'minimize' }, { role: 'zoom' }, { type: 'separator' }, { role: 'front' }]
    }
  ]
  return Menu.buildFromTemplate(template)
}

async function createWindow(): Promise<void> {
  mainWindow = new BrowserWindow({
    width: 1480,
    height: 940,
    minWidth: 1080,
    minHeight: 680,
    title: 'Bob Console',
    backgroundColor: '#0e1116',
    titleBarStyle: process.platform === 'darwin' ? 'hiddenInset' : 'default',
    trafficLightPosition: { x: 16, y: 18 },
    icon: windowIcon(),
    show: false,
    webPreferences: {
      preload: join(here, '../preload/index.mjs'),
      sandbox: false,
      contextIsolation: true,
      nodeIntegration: false,
      spellcheck: false
    }
  })

  mainWindow.once('ready-to-show', () => {
    if (!isSmoke) mainWindow?.show()
  })

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith('https://bobcli.dev') || url.startsWith('https://github.com/abdul-hamid-achik/bob')) {
      void shell.openExternal(url)
    }
    return { action: 'deny' }
  })
  mainWindow.webContents.on('will-navigate', (event, url) => {
    const devUrl = process.env.ELECTRON_RENDERER_URL
    if (!devUrl || !url.startsWith(devUrl)) event.preventDefault()
  })

  if (process.env.ELECTRON_RENDERER_URL) {
    await mainWindow.loadURL(process.env.ELECTRON_RENDERER_URL)
  } else {
    const file = join(here, '../renderer/index.html')
    if (isSmoke) console.log(`[smoke] loading ${file}`)
    mainWindow.webContents.on('did-fail-load', (_event, code, description) => {
      console.error(`[main] did-fail-load ${code} ${description}`)
    })
    await mainWindow.loadFile(file)
  }
  if (isSmoke) console.log('[smoke] renderer loaded')
}

const gotLock = app.requestSingleInstanceLock()
if (!gotLock) {
  app.quit()
} else {
  app.on('second-instance', () => {
    if (!mainWindow) return
    if (mainWindow.isMinimized()) mainWindow.restore()
    mainWindow.focus()
  })

  void app.whenReady().then(async () => {
    Menu.setApplicationMenu(buildMenu())
    if (process.platform === 'darwin' && !app.isPackaged && existsSync(devIconPath)) {
      app.dock?.setIcon(nativeImage.createFromPath(devIconPath))
    }
    state = await createState(app.getPath('userData'), here)
    const theme = state.settings.get().theme
    nativeTheme.themeSource = theme
    registerIpc({ state, getWindow: () => mainWindow })
    await createWindow()

    if (isSmoke && mainWindow && state) {
      const window = mainWindow
      const activeState = state
      const workspace = process.env.BOB_CONSOLE_SMOKE_WORKSPACE ?? activeState.settings.get().defaultWorkspace
      const reportPath = process.env.BOB_CONSOLE_SMOKE_REPORT ?? '/tmp/bob-console-smoke.json'
      if (!workspace) {
        console.error('[smoke] no workspace provided; set BOB_CONSOLE_SMOKE_WORKSPACE')
        app.exit(2)
      } else {
        // createWindow already awaited the load, so start from here rather than
        // waiting for a did-finish-load that has already fired.
        setTimeout(() => {
          void runSmoke(window, activeState, workspace, reportPath)
            .then((report) => {
              console.log(`[smoke] ${report.passed} passed, ${report.failed} failed → ${reportPath}`)
              app.exit(report.failed === 0 ? 0 : 1)
            })
            .catch((error) => {
              console.error('[smoke] crashed', error)
              app.exit(3)
            })
        }, 800)
      }
    }

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) void createWindow()
    })
  })
}

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})

app.on('before-quit', () => {
  state?.runner.stopAll()
  state?.mcp.stop()
})

process.on('uncaughtException', (error) => {
  console.error('[bob-console] uncaught', error)
})
