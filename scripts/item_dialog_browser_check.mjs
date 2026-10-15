#!/usr/bin/env node

// Browser acceptance for the Item Search container dialog. Character
// containers use real retained data over the live socket; guild-storage
// reference cases (exact 30 slots, partial pages, zero/unknown gold, late
// responses) use browser-only route fixtures labeled as such. No bot command
// or database write is issued.
import assert from 'node:assert/strict'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const origin = process.env.PHMON_ITEM_WEB_URL || 'http://127.0.0.1:3005'
const secret = process.env.OPERATOR_ACCESS_SECRET
assert(secret, 'Set OPERATOR_ACCESS_SECRET')
const directory = process.env.PHMON_ITEM_SCREENSHOTS || '/tmp/phmon-item-dialog'
mkdirSync(directory, { recursive: true })
const { chromium } = await import(process.env.PHMON_PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] })
const results = []

function guildFixture(server, guild, capacity, gold, items) {
  // Browser-only fixture: mirrors the /api/guild-storage response shape.
  return {
    server,
    guild,
    items: [{
      resource_key: 'guild_storage',
      availability: 'observed',
      observed_at: '2026-10-15T12:00:00Z',
      checked_at: '2026-10-15T12:00:00Z',
      observer_character_id: 'fixture-observer',
      observer_name: 'FixtureObserver',
      payload: {
        capacity,
        ...(gold === undefined ? {} : { gold }),
        slots: items.map((item, index) => ({ source_slot: index * 3, item })),
      },
    }],
  }
}

async function dialogGeometry(page) {
  return page.locator('.inventory-dialog').evaluate(dialog => {
    const grid = dialog.querySelector('.inventory-grid')
    const slot = dialog.querySelector('.item-slot')
    const box = dialog.getBoundingClientRect()
    return {
      width: Math.round(box.width),
      padding: getComputedStyle(dialog).paddingTop,
      columns: grid ? getComputedStyle(grid).gridTemplateColumns.split(' ').length : 0,
      slots: grid ? grid.querySelectorAll('.item-slot').length : 0,
      slot: slot ? Math.round(slot.getBoundingClientRect().width) : 0,
      gridFullWidth: grid ? Math.abs(grid.getBoundingClientRect().width - dialog.querySelector('.inventory-view').getBoundingClientRect().width) < 2 : false,
      pages: dialog.querySelectorAll('.inventory-pages button').length,
      gold: dialog.querySelector('.inventory-gold-bar')?.textContent.trim(),
      fitsViewport: box.right <= innerWidth && box.left >= 0 && box.bottom <= innerHeight,
    }
  })
}

try {
  for (const [width, height] of [[1440, 1000], [1280, 800], [390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height }, hasTouch: width === 390 })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    const frames = []
    page.on('websocket', socket => {
      socket.on('framesent', frame => {
        try { frames.push(JSON.parse(frame.payload)) } catch {}
      })
    })
    assert.equal((await page.request.post(`${origin}/api/auth/login`, { headers: { origin }, data: { secret } })).status(), 200)
    const realGuild = (await (await page.request.get(`${origin}/api/items?limit=100&include_sets=true`)).json()).items.filter(row => row.item?.model)
    const sampleItems = realGuild.slice(0, 4).map(row => row.item)
    await page.goto(`${origin}/items`)
    await page.waitForFunction(() => document.querySelector('.item-search-panel')?.getAttribute('aria-busy') === 'false')
    await page.locator('.item-search-tile').first().waitFor()
    const search = page.getByLabel('Search item name, code or model')
    await page.locator('.item-search-sets input').check()
    const urlBefore = page.url()
    const result = { width, height, dialogs: {} }

    for (const [name, title, columns] of [['Open Char Inventory', 'Inventory', 4], ['Open Char Storage', 'Storage', 6], ['Open Guild Storage', 'Guild Storage', 6]]) {
      const button = page.getByRole('button', { name, exact: true }).first()
      if (!(await button.count())) { result.dialogs[title] = 'no retained container'; continue }
      if (title === 'Guild Storage') {
        // Rapid target change: the first (delayed) response must be discarded.
        let call = 0
        await page.route('**/api/guild-storage?*', async route => {
          const url = new URL(route.request().url())
          const server = url.searchParams.get('server'), guild = url.searchParams.get('guild')
          call++
          if (call === 1) {
            await new Promise(resolve => setTimeout(resolve, 1500))
            return route.fulfill({ json: guildFixture(server, guild, 90, 999, sampleItems.slice(0, 1)) }).catch(() => {})
          }
          return route.fulfill({ json: guildFixture(server, guild, 30, 0, sampleItems) })
        })
        await button.click()
        await page.keyboard.press('Escape')
        await page.getByRole('dialog').waitFor({ state: 'detached' })
      }
      await button.scrollIntoViewIfNeeded()
      const scrollBefore = await page.evaluate(() => scrollY)
      await button.click()
      const dialog = page.getByRole('dialog', { name: title, exact: true })
      await dialog.waitFor()
      assert.equal(page.url(), urlBefore, 'Opening a container dialog must not navigate')
      await dialog.locator('.inventory-grid, .inventory-unavailable').first().waitFor()
      if (title === 'Guild Storage') await page.waitForTimeout(1800)
      const geometry = await dialogGeometry(page)
      if (geometry.slots) {
        assert.equal(geometry.columns, columns, `${title} columns`)
        assert.equal(geometry.slots, columns === 4 ? 32 : 30, `${title} slots per page`)
        assert.equal(geometry.padding, width === 390 ? '14px' : '20px')
        assert(geometry.gridFullWidth, 'Contents frame must be full-width')
        if (width > 430) assert.equal(geometry.slot, 44)
      }
      assert(geometry.fitsViewport, `${title} dialog exceeds the viewport`)
      if (title === 'Guild Storage') {
        assert.equal(geometry.pages, 1, 'Late 90-slot response must not replace the current 30-slot target')
        assert.equal(geometry.gold, '0', 'Known zero gold')
        assert.match(await dialog.locator('.dialog-header p').innerText(), /Viewed by: FixtureObserver/)
      }
      // Escape ordering: tooltip first, then the modal.
      const occupied = dialog.locator('.inventory-grid .item-slot:not(:disabled)').first()
      if (await occupied.count()) {
        if (width === 390) await occupied.tap()
        else await occupied.focus()
        const tooltip = page.locator('.item-tooltip.is-placed')
        await tooltip.waitFor()
        // The tooltip ignores pointer events, so compare stacking order of the
        // two fixed, body-level layers instead of hit testing.
        const above = await page.evaluate(() => Number(getComputedStyle(document.querySelector('.item-tooltip')).zIndex) > Number(getComputedStyle(document.querySelector('.inventory-dialog-layer')).zIndex))
        assert(above, 'Tooltip must render above the modal')
        if (width !== 390) {
          await page.keyboard.press('Escape')
          await tooltip.waitFor({ state: 'detached' })
          assert.equal(await dialog.count(), 1, 'First Escape closes only the tooltip')
        } else {
          await dialog.locator('.dialog-header h2').click()
        }
      }
      if (geometry.pages > 1) {
        await dialog.getByRole('button', { name: 'Page 2', exact: true }).click()
        assert.equal(await dialog.getByRole('button', { name: 'Page 2', exact: true }).getAttribute('aria-pressed'), 'true')
      }
      // Focus trap.
      for (let index = 0; index < 40; index++) await page.keyboard.press('Tab')
      assert(await dialog.evaluate(element => element.contains(document.activeElement)), 'Focus escaped the dialog')
      await page.screenshot({ path: join(directory, `${title.replaceAll(' ', '-').toLowerCase()}-${width}x${height}.png`) })
      // Tab may have focused an item and opened its tooltip; that Escape
      // closes the tooltip and a second Escape closes the dialog.
      if (await page.locator('.item-tooltip').count()) {
        await page.keyboard.press('Escape')
        await page.locator('.item-tooltip').waitFor({ state: 'detached' })
        assert.equal(await dialog.count(), 1)
      }
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'detached' })
      assert.equal(await page.evaluate(() => document.activeElement?.textContent?.trim()), name, 'Focus returns to the opener')
      assert(Math.abs((await page.evaluate(() => scrollY)) - scrollBefore) < 4, 'Scroll position preserved')
      assert(await page.locator('.item-search-sets input').isChecked(), 'Filters preserved')
      result.dialogs[title] = geometry
      if (title === 'Guild Storage') await page.unroute('**/api/guild-storage?*')
    }

    // Partial final page and unknown gold (browser-only fixture), backdrop close.
    const guildButton = page.getByRole('button', { name: 'Open Guild Storage', exact: true }).first()
    if (await guildButton.count()) {
      await page.route('**/api/guild-storage?*', route => {
        const url = new URL(route.request().url())
        return route.fulfill({ json: guildFixture(url.searchParams.get('server'), url.searchParams.get('guild'), 45, undefined, sampleItems) })
      })
      await guildButton.click()
      const dialog = page.getByRole('dialog', { name: 'Guild Storage', exact: true })
      await dialog.locator('.inventory-grid').waitFor()
      const partial = await dialogGeometry(page)
      assert.equal(partial.pages, 2)
      assert.equal(partial.gold, '—', 'Unknown gold shows a dash')
      await dialog.getByRole('button', { name: 'Page 2', exact: true }).click()
      assert.equal(await dialog.locator('.inventory-grid > *').count(), 30, 'Page geometry stays 6 × 5')
      await page.mouse.click(5, 5)
      await dialog.waitFor({ state: 'detached' })
      await page.unroute('**/api/guild-storage?*')
      result.partialGuildPage = true
    }

    // Subscription cleanup: every dialog subscribe has a matching unsubscribe.
    const subscribed = new Set(frames.filter(frame => frame.type === 'subscribe' && String(frame.subscription_id).startsWith('resource-dialog-')).map(frame => frame.subscription_id))
    const unsubscribed = new Set(frames.filter(frame => frame.type === 'unsubscribe' && String(frame.subscription_id).startsWith('resource-dialog-')).map(frame => frame.subscription_id))
    for (const id of subscribed) assert(unsubscribed.has(id), `Dialog subscription ${id} was not released`)
    result.dialogSubscriptions = subscribed.size
    assert.equal(await search.inputValue(), '')
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)

    // Inline regression: the character card keeps 4 × 38 px slots.
    const characterLink = await page.request.get(`${origin}/api/items?limit=1`)
    const characterID = (await characterLink.json()).items[0]?.observer_character_id
    if (characterID) {
      await page.goto(`${origin}/characters/${characterID}?resource=inventory`)
      const inline = page.locator('.inventory-view:not(.is-dialog) .inventory-grid').first()
      await inline.waitFor()
      const inlineGeometry = await inline.evaluate(grid => ({ columns: getComputedStyle(grid).gridTemplateColumns.split(' ').length, slot: Math.round(grid.querySelector('.item-slot').getBoundingClientRect().width) }))
      assert.deepEqual(inlineGeometry, { columns: 4, slot: 38 })
      result.inlineInventory = inlineGeometry
    }
    assert.deepEqual(errors, [])
    result.browserErrors = 0
    results.push(result)
    await context.close()
  }
} catch (error) {
  const failedPage = browser.contexts().at(-1)?.pages().at(-1)
  if (failedPage) await failedPage.screenshot({ path: join(directory, 'dialog-failure.png') }).catch(() => {})
  throw error
} finally {
  await browser.close()
}
writeFileSync(join(directory, 'dialog-results.json'), `${JSON.stringify(results, null, 2)}\n`)
console.log(JSON.stringify(results, null, 2))
