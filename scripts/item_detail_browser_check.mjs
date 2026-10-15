#!/usr/bin/env node

// Read-only acceptance against retained item observations. No commands or fixtures
// are sent to phBot. Configure an isolated or local instance and operator secret.
import assert from 'node:assert/strict'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const origin = process.env.PHMON_ITEM_WEB_URL || 'http://127.0.0.1:3005'
const secret = process.env.OPERATOR_ACCESS_SECRET
assert(secret, 'Set OPERATOR_ACCESS_SECRET')
const directory = process.env.PHMON_ITEM_SCREENSHOTS || '/tmp/phmon-item-details'
mkdirSync(directory, { recursive: true })
const { chromium } = await import(process.env.PHMON_PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] })
const results = []
try {
  for (const [width, height] of [[1440, 1000], [1280, 800], [390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height }, hasTouch: width === 390, timezoneId: 'Europe/Amsterdam' })
    const page = await context.newPage()
    const errors = []
    const liveFrames = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('websocket', socket => socket.on('framereceived', ({ payload }) => {
      try {
        const frame = JSON.parse(String(payload))
        liveFrames.push({ type: frame.type, subscription: frame.subscription_id, error: frame.error, events: frame.data?.events?.length })
      } catch { /* Ignore non-JSON transport data. */ }
    }))
    const login = await page.request.post(`${origin}/api/auth/login`, {
      headers: { origin }, data: { secret },
    })
    assert.equal(login.status(), 200)
    for (const kind of ['drop.item', 'drop.rare']) {
      const response = await page.request.get(`${origin}/api/events?kind=${kind}&limit=100`)
      assert.equal(response.status(), 200)
      const data = await response.json()
      assert(data.events.length, `No retained ${kind} events to verify`)
      assert(data.events.every(event => event.kind === kind && event.source === 'phbot.callback'), 'Drop defaults included non-callback gains')
      const event = data.events.find(event => event.related_item_observation?.item?.instance_details?.stats?.length && event.related_item_observation?.item?.instance_details?.blues?.length)
      assert(event, `Need a retained ${kind} with separately observed stats and blues`)
      const observation = event.related_item_observation
      assert.equal(event.payload.item, undefined, 'Related evidence must not become the callback item')
      assert.equal(event.item_details, undefined)
      const sourceResponse = await page.request.get(`${origin}/api/events?event_id=${observation.event_id}`)
      const source = (await sourceResponse.json()).events[0]
      assert.equal(source.event_id, observation.event_id)
      assert.equal(source.character_id, event.character_id)
      assert.equal(source.session_id, event.session_id)
      assert.deepEqual(observation.item.instance_details, source.item_details.instance_details, 'Related stats differ from source event')
      await page.goto(`${origin}/events?kind=${kind}&event_id=${event.event_id}`)
      console.log(JSON.stringify({ width, height, kind, eventID: event.event_id, phase: 'loaded' }))
      const item = page.locator('.drop-history-item .item-detail-name').first()
      try {
        await item.waitFor()
      } catch (error) {
        await page.screenshot({ path: join(directory, `failure-${kind}-${width}.png`), fullPage: true })
        console.error((await page.locator('body').innerText()).slice(-3000))
        console.error(JSON.stringify({ errors, frames: liveFrames.slice(-15) }))
        throw error
      }
      if (width === 390) await page.getByRole('button', { name: /^Show details for/ }).first().tap()
      else await item.focus()
      const tooltip = page.getByRole('tooltip')
      await tooltip.waitFor()
      await page.locator('.item-tooltip.is-placed').waitFor()
      const text = await tooltip.innerText()
      assert.equal(await tooltip.locator('.item-tooltip-name').count(), 1, 'Popup repeated the item identity')
      assert.equal(await tooltip.locator('.item-tooltip-caption').count(), 1)
      assert((await tooltip.locator('.item-tooltip-caption').getAttribute('title')).includes(observation.event_id))
      assert(!text.includes('Rolled stats and blue options were not observed'))
      assert(!text.includes('Source event:'))
      assert(!text.includes('Pet inventory observation'))
      assert(!text.includes('Inventory observation'))
      for (const stat of observation.item.instance_details.stats) assert(text.includes(`${stat.label} ${stat.value}`), `Missing observed stat ${stat.key}`)
      assert(await tooltip.locator('.item-tooltip-blue').count() > 0, 'Observed blues disappeared')
      if (width > 640) {
        assert.equal(Math.round((await item.locator('xpath=ancestor::tr').boundingBox()).height), 67)
        assert.equal(Math.round((await page.locator('.drop-history-item .item-detail-icon').first().boundingBox()).width), 45)
      }
      const rect = await tooltip.boundingBox()
      assert(rect && rect.x >= 0 && rect.y >= 0 && rect.x + rect.width <= width && rect.y + rect.height <= height, `Tooltip out of viewport: ${JSON.stringify(rect)}`)
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, 'Page overflow')
      await page.screenshot({ path: join(directory, `${kind}-${width}x${height}.png`), fullPage: true })
      await page.keyboard.press('Escape')
      await tooltip.waitFor({ state: 'detached' })
      results.push({ width, height, kind, eventID: event.event_id, observationID: observation.event_id, stats: observation.item.instance_details.stats.length, blues: observation.item.instance_details.blues.length, canonicalPayloadPreserved: true, keyboardAndEscape: true })
    }
    assert.deepEqual(errors, [], 'Browser errors')
    await context.close()
  }
} finally {
  await browser.close()
}
writeFileSync(join(directory, 'results.json'), `${JSON.stringify(results, null, 2)}\n`)
console.log(JSON.stringify(results))
