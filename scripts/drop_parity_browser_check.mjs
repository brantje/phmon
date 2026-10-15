#!/usr/bin/env node

// Read-only screenshot and interaction acceptance using retained source events.
// No bot command, monitoring fixture, or canonical history write is performed.
import assert from 'node:assert/strict'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const origin = process.env.PHMON_ITEM_WEB_URL || 'http://127.0.0.1:3005'
const secret = process.env.OPERATOR_ACCESS_SECRET
assert(secret, 'Set OPERATOR_ACCESS_SECRET')
const directory = process.env.PHMON_ITEM_SCREENSHOTS || '/tmp/phmon-drop-parity'
mkdirSync(directory, { recursive: true })
const { chromium } = await import(process.env.PHMON_PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] })
const results = []
try {
  for (const [width, height] of [[1440, 1000], [1280, 800], [390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height }, hasTouch: width === 390, timezoneId: 'Europe/Amsterdam' })
    const page = await context.newPage(), errors = [], offerSnapshots = [], sent = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('websocket', socket => {
      socket.on('framereceived', ({payload}) => { try { const frame = JSON.parse(String(payload)); if (frame.subscription_id === 'dashboard-offers' && frame.type === 'snapshot') offerSnapshots.push(frame.data) } catch {} })
      socket.on('framesent', ({payload}) => { try { sent.push(JSON.parse(String(payload))) } catch {} })
    })
    assert.equal((await page.request.post(`${origin}/api/auth/login`, { headers: { origin }, data: { secret } })).status(), 200)
    await page.goto(`${origin}/`)
    const offers = page.getByRole('article', { name: 'Recent Global offers' })
    await offers.locator('li').first().waitFor()
    assert(!(await offers.innerText()).includes('LATER'))
    assert.equal(await offers.getByRole('link', { name: /Show all offers/ }).getAttribute('href'), '/economy')
    assert.equal(await offers.getAttribute('aria-busy'), 'false')
    assert(offerSnapshots.length, 'Dashboard did not consume the shared live offers stream')
    assert(offerSnapshots.at(-1).offers.every(offer => ['WTB','WTS','WTT'].includes(offer.offer_type) && offer.source_event_id))
    assert.equal(await offers.locator('li').count(), offerSnapshots.at(-1).offers.length)
    if (width > 640) {
      const scope = page.getByRole('combobox', { name: 'Server scope' })
      if (await scope.count() && await scope.locator('option[value="Greatest"]').count()) {
        await scope.selectOption('Greatest')
        await page.waitForFunction(() => document.querySelector('[aria-label="Recent Global offers"]')?.getAttribute('aria-busy') === 'false')
        assert(sent.some(frame => frame.stream === 'offers' && frame.filter?.server === 'Greatest'), 'Offers did not revise their server scope')
        await scope.selectOption('all')
        await page.waitForFunction(() => { const panel = document.querySelector('[aria-label="Recent Global offers"]'); return panel?.getAttribute('aria-busy') === 'false' && panel.querySelectorAll('li').length > 0 })
      }
    }
    await offers.screenshot({ path: join(directory, `global-offers-${width}x${height}.png`) })
    await page.goto(`${origin}/events?kind=drop.rare`)
    const table = page.getByRole('table', { name: 'Rare drop observations' })
    await table.locator('.item-detail-name').first().waitFor()
    assert.equal(await page.getByText('All rare drops', { exact: true }).count(), 0)
    assert.equal(await table.locator('thead th').count(), 5)
    const first = table.locator('tbody tr').first()
    const geometry = await first.evaluate(row => ({ height: row.getBoundingClientRect().height, icon: row.querySelector('.item-detail-icon').getBoundingClientRect().width, font: getComputedStyle(row.querySelector('td')).fontSize }))
    assert.equal(Math.round(geometry.height), 67)
    assert.equal(Math.round(geometry.icon), 45)
    assert.equal(geometry.font, '13px')
    assert(/^[A-Z][a-z]{2} \d{1,2}, \d{4},/.test(await first.locator('time').innerText()), 'Drop date still uses raw UTC/Z formatting')
    if (width > 640) {
      const input = await page.getByLabel('Filter events by item').boundingBox()
      assert(Math.abs(input.width - 184) < 2, 'Reference-sized item filter stretched across the workspace')
    }
    await page.getByLabel('Filter events by item').fill('Gem Necklace')
    await page.waitForFunction(() => {
      const names = [...document.querySelectorAll('.drop-history-item .item-detail-name')].map(el => el.textContent.trim())
      return names.length > 0 && names.every(name => name.includes('Gem Necklace'))
    })
    await page.getByLabel('Filter events by item').fill('')
    await page.waitForFunction(() => [...document.querySelectorAll('.drop-history-table tbody .item-detail-name')].some(el => !el.textContent.includes('Gem Necklace')))
    const popupTarget = table.locator('.item-detail-name').first()
    if (width === 390) await table.getByRole('button', {name: /^Show details for/}).first().tap()
    else await popupTarget.focus()
    const tooltip = page.getByRole('tooltip')
    await page.locator('.item-tooltip.is-placed').waitFor()
    assert.equal(await tooltip.locator('.item-tooltip-name').count(), 1)
    assert(!(await tooltip.innerText()).includes('Rolled stats and blue options were not observed'))
    assert.equal(await tooltip.locator('.item-tooltip-observation').count(), 0)
    await page.screenshot({ path: join(directory, `rare-drops-${width}x${height}.png`), fullPage: true })
    await tooltip.screenshot({ path: join(directory, `rare-tooltip-${width}x${height}.png`) })
    await page.keyboard.press('Escape')
    await tooltip.waitFor({ state: 'detached' })
    if (process.env.PHMON_GEM_EVENT_ID) {
      await page.goto(`${origin}/events?event_id=${process.env.PHMON_GEM_EVENT_ID}`)
      const gem = page.locator('.event-row-primary .item-detail-name').filter({hasText: 'Gem Necklace'}).first()
      await gem.waitFor()
      if (width === 390) await page.getByRole('button', {name: 'Show details for Gem Necklace', exact: true}).first().tap()
      else await gem.focus()
      await page.locator('.item-tooltip.is-placed').waitFor()
      assert.equal(await tooltip.locator('.item-tooltip-name').count(), 1)
      assert.equal(await tooltip.locator('.item-tooltip-stat').count(), 2)
      assert.equal(await tooltip.locator('.item-tooltip-blue').count(), 2)
      assert.equal(await tooltip.locator('.item-tooltip-caption').count(), 0)
      await tooltip.screenshot({path: join(directory, `gem-necklace-${width}x${height}.png`)})
      await page.keyboard.press('Escape')
    }
    if (process.env.PHMON_PET_DROP_EVENT_ID) {
      await page.goto(`${origin}/events?kind=drop.item&event_id=${process.env.PHMON_PET_DROP_EVENT_ID}`)
      const pet = page.locator('.item-detail-name').first()
      await pet.waitFor()
      if (width === 390) await page.getByRole('button', {name: /^Show details for/}).first().tap()
      else await pet.focus()
      await page.locator('.item-tooltip.is-placed').waitFor()
      assert.equal(await tooltip.locator('.item-tooltip-name').count(), 1)
      assert.deepEqual(await tooltip.locator('.item-tooltip-stat').allTextContents(), ['Durability 82'])
      assert.equal(await tooltip.locator('.item-tooltip-blue').count(), 0)
      assert.equal(await tooltip.locator('.item-tooltip-caption').innerText(), 'Observed item · stats unavailable')
      await tooltip.screenshot({path: join(directory, `pet-basic-item-${width}x${height}.png`)})
      await page.keyboard.press('Escape')
    }
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
    assert.deepEqual(errors, [])
    results.push({width,height,geometry,offers:offerSnapshots.at(-1).offers.length,liveOffers:true,nameFilter:true,singleItemPopup:true,gem:!!process.env.PHMON_GEM_EVENT_ID,petGap:!!process.env.PHMON_PET_DROP_EVENT_ID,browserErrors:errors})
    await context.close()
  }
} finally { await browser.close() }
writeFileSync(join(directory,'parity-results.json'),JSON.stringify(results,null,2)+'\n')
console.log(JSON.stringify(results))
