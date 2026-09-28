import type { BobApi } from '@shared/ipc'
import { demoBridge } from './demoBridge'

let forcedDemo = false

export function setForcedDemo(value: boolean): void {
  forcedDemo = value
}

export function hasNativeBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.bob)
}

/** The bridge every panel talks to: the preload API, or the demo replayer. */
export function bridge(): BobApi {
  if (forcedDemo || !hasNativeBridge()) return demoBridge
  return window.bob
}

export function isDemoActive(): boolean {
  return forcedDemo || !hasNativeBridge()
}
