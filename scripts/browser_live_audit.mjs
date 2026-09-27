#!/usr/bin/env node

import { execFileSync, spawn } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const WEB_URL = (process.env.SMOKE_WEB_URL || 'http://127.0.0.1:3005').replace(/\/$/, '')
const OPERATOR_ACCESS_SECRET = process.env.OPERATOR_ACCESS_SECRET || ''
const READY_FILE = process.env.BROWSER_AUDIT_READY_FILE || ''
const REQUIRE_RECONNECT = process.env.BROWSER_AUDIT_REQUIRE_RECONNECT === '1'
const TIMEOUT_MS = Number(process.env.BROWSER_AUDIT_TIMEOUT_MS || 60000)

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function findChrome() {
  if (process.env.CHROME_PATH) return process.env.CHROME_PATH
  for (const command of ['google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser']) {
    try {
      return execFileSync('which', [command], { encoding: 'utf8' }).trim()
    } catch {
      // Try the next known binary.
    }
  }
  throw new Error('No Chrome/Chromium binary found for browser network audit')
}

async function waitFor(predicate, description, timeout = TIMEOUT_MS) {
  const deadline = Date.now() + timeout
  let lastError
  while (Date.now() < deadline) {
    try {
      const value = await predicate()
      if (value) return value
    } catch (error) {
      lastError = error
    }
    await sleep(100)
  }
  throw new Error(
    `Timed out waiting for ${description}${lastError ? `: ${lastError.message}` : ''}`,
  )
}

class CDP {
  constructor(socket) {
    this.socket = socket
    this.nextID = 1
    this.pending = new Map()
    this.listeners = new Map()
    socket.addEventListener('message', (event) => {
      const message = JSON.parse(String(event.data))
      if (message.id) {
        const pending = this.pending.get(message.id)
        if (!pending) return
        this.pending.delete(message.id)
        if (message.error) pending.reject(new Error(message.error.message))
        else pending.resolve(message.result || {})
        return
      }
      for (const listener of this.listeners.get(message.method) || []) {
        listener(message.params || {})
      }
    })
    socket.addEventListener('close', () => {
      for (const pending of this.pending.values()) {
        pending.reject(new Error('Chrome DevTools connection closed'))
      }
      this.pending.clear()
    })
  }

  static async connect(url) {
    const socket = new WebSocket(url)
    await new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true })
      socket.addEventListener('error', () => reject(new Error('CDP WebSocket failed')), {
        once: true,
      })
    })
    return new CDP(socket)
  }

  send(method, params = {}) {
    const id = this.nextID++
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.socket.send(JSON.stringify({ id, method, params }))
    })
  }

  on(method, listener) {
    const listeners = this.listeners.get(method) || []
    listeners.push(listener)
    this.listeners.set(method, listeners)
  }

  close() {
    this.socket.close()
  }
}

async function evaluate(cdp, expression) {
  const result = await cdp.send('Runtime.evaluate', {
    expression,
    awaitPromise: true,
    returnByValue: true,
  })
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || 'browser evaluation failed')
  }
  return result.result?.value
}

function forbiddenLiveRead(url, method, type) {
  if (type === 'EventSource') return `SSE request ${method} ${url}`
  let parsed
  try {
    parsed = new URL(url)
  } catch {
    return null
  }
  if (method !== 'GET') return null
  if (parsed.pathname === '/api/agents') return `HTTP live read ${method} ${parsed.pathname}`
  if (parsed.pathname === '/api/groups') return `HTTP live read ${method} ${parsed.pathname}`
  if (
    parsed.pathname === '/api/characters' ||
    parsed.pathname.startsWith('/api/characters/')
  ) {
    return `HTTP live read ${method} ${parsed.pathname}`
  }
  return null
}

async function main() {
  if (!OPERATOR_ACCESS_SECRET) {
    throw new Error('OPERATOR_ACCESS_SECRET is required for the authenticated browser audit')
  }
  const chrome = findChrome()
  const profile = mkdtempSync(join(tmpdir(), 'phmon-chrome-'))
  const stderr = []
  const child = spawn(
    chrome,
    [
      '--headless=new',
      '--disable-gpu',
      '--no-sandbox',
      '--disable-dev-shm-usage',
      '--remote-debugging-address=127.0.0.1',
      '--remote-debugging-port=0',
      `--user-data-dir=${profile}`,
      'about:blank',
    ],
    { stdio: ['ignore', 'ignore', 'pipe'] },
  )
  child.stderr.on('data', (chunk) => stderr.push(String(chunk)))

  let cdp
  try {
    const portFile = join(profile, 'DevToolsActivePort')
    try {
      await waitFor(() => existsSync(portFile), 'Chrome DevTools port', 30000)
    } catch (error) {
      throw new Error(`${error.message}\n${stderr.join('')}`)
    }
    const [port] = readFileSync(portFile, 'utf8').trim().split(/\s+/)
    const targets = await fetch(`http://127.0.0.1:${port}/json/list`).then((response) =>
      response.json(),
    )
    const page = targets.find((target) => target.type === 'page')
    if (!page?.webSocketDebuggerUrl) throw new Error('Chrome page target unavailable')
    cdp = await CDP.connect(page.webSocketDebuggerUrl)

    const forbidden = []
    const liveSocketURLs = new Map()
    let successfulLiveSockets = 0

    cdp.on('Network.requestWillBeSent', ({ request, type }) => {
      const violation = forbiddenLiveRead(request.url, request.method, type)
      if (violation) forbidden.push(violation)
    })
    cdp.on('Network.webSocketCreated', ({ requestId, url }) => {
      if (new URL(url).pathname === '/api/live') {
        liveSocketURLs.set(requestId, url)
      }
    })
    cdp.on('Network.webSocketHandshakeResponseReceived', ({ requestId, response }) => {
      if (liveSocketURLs.has(requestId) && response.status === 101) {
        successfulLiveSockets += 1
      }
    })

    await Promise.all([
      cdp.send('Network.enable'),
      cdp.send('Page.enable'),
      cdp.send('Runtime.enable'),
    ])
    await cdp.send('Page.navigate', { url: WEB_URL + '/' })
    await waitFor(
      () => evaluate(cdp, `document.readyState === 'complete'`),
      'operator sign-in shell',
    )
    const login = await evaluate(
      cdp,
      `fetch(${JSON.stringify(WEB_URL + '/api/auth/login')}, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ secret: ${JSON.stringify(OPERATOR_ACCESS_SECRET)} }),
      }).then(async (response) => ({ ok: response.ok, body: await response.json() }))`,
    )
    if (!login?.ok || login.body?.authenticated !== true) {
      throw new Error('operator login failed in browser audit')
    }
    await cdp.send('Page.navigate', { url: WEB_URL + '/' })

    await waitFor(
      () => successfulLiveSockets >= 1,
      'successful same-origin /api/live WebSocket',
      20000,
    )
    await waitFor(
      () =>
        evaluate(
          cdp,
          `document.body && document.body.innerText.includes('phBot agents') && document.body.innerText.includes('Characters')`,
        ),
      'dashboard rendering',
      20000,
    )

    // Search filtering must only revise the characters subscription.
    await evaluate(
      cdp,
      `(() => {
        const input = document.querySelector('input[aria-label="Search characters, guild, server or zone"]')
        if (!input) throw new Error('character search input missing')
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set
        setter.call(input, 'Fixture')
        input.dispatchEvent(new Event('input', { bubbles: true }))
        return true
      })()`,
    )
    await sleep(700)

    // Manual refresh buttons must send protocol refresh frames, never HTTP reads.
    const refreshCount = await evaluate(
      cdp,
      `(() => {
        const buttons = [...document.querySelectorAll('button')].filter((button) => button.textContent.trim() === 'Refresh')
        buttons.forEach((button) => button.click())
        return buttons.length
      })()`,
    )
    if (refreshCount < 2) throw new Error(`expected dashboard refresh controls, found ${refreshCount}`)
    await sleep(500)

    // Exercise an HTTP mutation and wait for its state to arrive over WebSocket.
    const groupName = `browser-audit-${Date.now()}`
    await evaluate(
      cdp,
      `(() => {
        const input = document.querySelector('input[aria-label="Character group name"]')
        if (!input) throw new Error('group name input missing')
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set
        setter.call(input, ${JSON.stringify(groupName)})
        input.dispatchEvent(new Event('input', { bubbles: true }))
        const button = [...document.querySelectorAll('button')].find((item) => item.textContent.trim() === 'New group')
        if (!button) throw new Error('New group button missing')
        button.click()
        return true
      })()`,
    )
    await waitFor(
      () =>
        evaluate(
          cdp,
          `[...document.querySelectorAll('select[aria-label="Filter by character group"] option')].some((option) => option.textContent.trim() === ${JSON.stringify(groupName)})`,
        ),
      'group mutation replacement snapshot',
      10000,
    )

    // Select the group to exercise filtered character subscription semantics.
    await evaluate(
      cdp,
      `(() => {
        const select = document.querySelector('select[aria-label="Filter by character group"]')
        const option = [...select.options].find((item) => item.textContent.trim() === ${JSON.stringify(groupName)})
        if (!option) throw new Error('new group option missing')
        const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value').set
        setter.call(select, option.value)
        select.dispatchEvent(new Event('change', { bubbles: true }))
        return true
      })()`,
    )
    await sleep(500)

    // Delete is another action response that must not act as a refresh mechanism.
    await evaluate(
      cdp,
      `(() => {
        window.confirm = () => true
        const button = [...document.querySelectorAll('button')].find((item) => item.textContent.trim() === 'Delete')
        if (!button) throw new Error('Delete group button missing')
        button.click()
        return true
      })()`,
    )
    await waitFor(
      () =>
        evaluate(
          cdp,
          `![...document.querySelectorAll('select[aria-label="Filter by character group"] option')].some((option) => option.textContent.trim() === ${JSON.stringify(groupName)})`,
        ),
      'group deletion replacement snapshot',
      10000,
    )

    // Verify the current responsive shell still fits both mobile and desktop widths.
    for (const [width, height] of [
      [390, 844],
      [1440, 1000],
    ]) {
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width,
        height,
        deviceScaleFactor: 1,
        mobile: false,
      })
      await sleep(250)
      const fits = await evaluate(
        cdp,
        `document.documentElement.scrollWidth <= window.innerWidth + 1`,
      )
      if (!fits) throw new Error(`responsive layout overflows at ${width}x${height}`)
      if (width === 390) {
        const characterTable = await evaluate(
          cdp,
          `(() => {
            const table = document.querySelector('.character-table')
            if (!table) return { present: false }
            return {
              present: true,
              visible: getComputedStyle(table).display !== 'none',
              rows: table.querySelectorAll('tbody tr').length,
              scrollsInsidePanel: table.closest('.agent-table-wrap')?.scrollWidth >= table.closest('.agent-table-wrap')?.clientWidth,
            }
          })()`,
        )
        if (!characterTable.present || !characterTable.visible || characterTable.rows === 0) {
          throw new Error(`character list is missing at ${width}x${height}: ${JSON.stringify(characterTable)}`)
        }
        if (!characterTable.scrollsInsidePanel) {
          throw new Error(`character table has no bounded scroll region at ${width}x${height}`)
        }
      }
    }

    // Navigate to a streamed stable character detail when fixture/history data exists.
    const detailURL = await evaluate(
      cdp,
      `document.querySelector('a.character-link')?.href || ''`,
    )
    if (detailURL) {
      await cdp.send('Page.navigate', { url: detailURL })
      await waitFor(
        () =>
          evaluate(
            cdp,
            `document.querySelector('.character-detail-view') !== null`,
          ),
        'character detail live view',
        15000,
      )
    }

    if (forbidden.length) {
      throw new Error(`forbidden browser live-data traffic: ${[...new Set(forbidden)].join('; ')}`)
    }

    const baselineSockets = successfulLiveSockets
    if (READY_FILE) writeFileSync(READY_FILE, 'ready\n')

    if (REQUIRE_RECONNECT) {
      const login = await evaluate(
        cdp,
        `fetch(${JSON.stringify(WEB_URL + '/api/auth/login')}, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ secret: ${JSON.stringify(OPERATOR_ACCESS_SECRET)} }),
        }).then(async (response) => ({ ok: response.ok, body: await response.json() }))`,
      )
      if (!login?.ok || login.body?.authenticated !== true) {
        throw new Error('operator reauthentication failed after backend restart')
      }
      await cdp.send('Page.reload', { ignoreCache: true })
      await waitFor(
        () => evaluate(cdp, `document.readyState === 'complete'`),
        'application reload after operator reauthentication',
      )
      await waitFor(
        () => successfulLiveSockets > baselineSockets,
        'browser WebSocket reconnect after backend restart',
        TIMEOUT_MS,
      )
      await waitFor(
        async () => {
          const text = await evaluate(cdp, 'document.body?.innerText || ""')
          return !text.includes('Live data stale') && !text.includes('retrying the WebSocket')
        },
        'fresh snapshots after browser reconnect',
        TIMEOUT_MS,
      )
    }

    // Give delayed fetches/pollers a chance to betray themselves before the final audit.
    await sleep(1200)
    if (forbidden.length) {
      throw new Error(`forbidden browser live-data traffic: ${[...new Set(forbidden)].join('; ')}`)
    }

    console.log(
      `Browser live-data audit passed: ${successfulLiveSockets} successful /api/live WebSocket connection(s), zero HTTP/SSE live-data reads across startup, filtering, manual refresh, mutations${REQUIRE_RECONNECT ? ', backend restart/recovery' : ''}, and responsive layout checks.`,
    )
  } finally {
    try {
      cdp?.close()
    } catch {
      // Best-effort cleanup.
    }
    child.kill('SIGTERM')
    await Promise.race([
      new Promise((resolve) => child.once('exit', resolve)),
      sleep(3000),
    ])
    if (child.exitCode == null) child.kill('SIGKILL')
    rmSync(profile, { recursive: true, force: true })
    if (child.exitCode && child.exitCode !== 0) {
      console.error(stderr.join(''))
    }
  }
}

main().catch((error) => {
  console.error(error.stack || error)
  process.exit(1)
})
