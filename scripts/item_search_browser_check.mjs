#!/usr/bin/env node

// Exercise the real browser -> Nuxt proxy -> Go -> PostgreSQL read path.
// No monitoring fixtures, bot commands or database writes are introduced.
import assert from 'node:assert/strict'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const origin = process.env.PHMON_ITEM_WEB_URL || 'http://127.0.0.1:3005'
const secret = process.env.OPERATOR_ACCESS_SECRET
assert(secret, 'Set OPERATOR_ACCESS_SECRET')
const directory = process.env.PHMON_ITEM_SCREENSHOTS || '/tmp/phmon-item-search'
mkdirSync(directory, { recursive: true })
const { chromium } = await import(process.env.PHMON_PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] })
const results = []
// Item Search sends static presentation once, in item.metadata.
const itemMetadata = row => row.metadata || row.item.metadata || {}
try {
  const anonymous = await browser.newContext()
  for (const path of ['/api/items', '/api/economy/offers']) assert.equal((await anonymous.request.get(`${origin}${path}`)).status(), 401, `${path} must require authentication`)
  await anonymous.close()
  for (const [width, height] of [[1440, 1000], [1280, 800], [390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height }, hasTouch: width === 390 })
    const page = await context.newPage()
    const itemPages = []
    page.on('response', async response => {
      if (new URL(response.url()).pathname === '/api/items' && new URL(response.url()).searchParams.get('limit') === '5000' && response.status() === 200) {
        const data = await response.json()
        if (!new URL(response.url()).searchParams.has('cursor')) itemPages.length = 0
        itemPages.push(data)
      }
    })
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    assert.equal((await page.request.post(`${origin}/api/auth/login`, { headers: { origin }, data: { secret } })).status(), 200)
    const response = await page.request.get(`${origin}/api/items?limit=1`)
    assert.equal(response.status(), 200, 'Item Search proxy is unavailable')
    assert.equal(response.headers()['cache-control'], 'no-store')
    const first = await response.json()
    assert(first.total > 0 && first.items.length === 1, 'Need retained real items for browser acceptance')
    assert.equal((await page.request.get(`${origin}/api/items?limit=5001`)).status(), 400, 'Backend validation status must survive the proxy')
    const emptyResponse = await page.request.get(`${origin}/api/items?q=PHMON_NO_SUCH_ITEM_20261015`)
    assert.equal(emptyResponse.status(), 200)
    assert.equal((await emptyResponse.json()).total, 0, 'Search query was not forwarded')
    if (first.next_cursor) {
      const next = await page.request.get(`${origin}/api/items?limit=1&cursor=${encodeURIComponent(first.next_cursor)}`)
      assert.equal(next.status(), 200)
      const second = (await next.json()).items[0]
      assert(second)
      const key = item => [item.server, item.observer_character_id, item.container, item.pet_id, item.slot].join('/')
      assert.notEqual(key(second), key(first.items[0]), 'Cursor was not forwarded')
    }
    await page.goto(`${origin}/items`)
        await page.waitForFunction(() => document.querySelector('.item-search-panel')?.getAttribute('aria-busy') === 'false')
    const tiles = page.locator('.item-search-tile')
    await tiles.first().waitFor()
    assert.equal(await page.getByRole('alert').count(), 0)
    assert(itemPages.length > 0 && !itemPages.at(-1).next_cursor, 'Expected a complete item load, not a partial API page')
    const retained = new Map(itemPages.flatMap(result => result.items).map(item => [[item.server.toLowerCase(), item.observer_character_id, item.container, item.pet_id || '', item.slot].join('/'), item]))
    const rows = [...retained.values()]
    const defaultRows = rows.filter(row => row.container !== 'equipment')
    assert.equal(await tiles.count(), defaultRows.length)
    assert.equal(Number(await page.getByLabel('Total matching items').innerText()), defaultRows.length)
    for (const owner of await page.locator('.item-search-owner').all()) {
      const count = Number((await owner.locator('.item-search-owner-heading').first().locator('.item-search-count').innerText()).split(' ')[0])
      assert.equal(count, await owner.locator('.item-search-tile').count(), 'Owner match count is partial')
    }
    for (const container of await page.locator('.item-search-container').all()) {
      const count = Number((await container.locator('.item-search-count').innerText()).split(' ')[0])
      assert.equal(count, await container.locator('.item-search-tile').count(), 'Container match count is partial')
    }
    assert.equal(await page.getByText('Observed by:', { exact: false }).count(), 0)
    assert.equal(await page.getByText('View canonical item events', { exact: true }).count(), 0)
    const search = page.getByLabel('Search item name, code or model')
    await search.fill('PHMON_NO_SUCH_ITEM_20261015')
    await page.getByText('No matching items', { exact: true }).waitFor()
    await page.getByRole('button', { name: 'Reset Filters', exact: true }).click()
    await tiles.first().waitFor()
    await page.locator('.item-search-sets input').check()
    assert.equal(await tiles.count(), rows.length)
    const weapons = rows.filter(row => !String(row.item.servername || '').startsWith('ITEM_MALL_') && itemMetadata(row).sort_type === 'Weapon')
    await page.getByLabel('Item Type', { exact: true }).selectOption('weapons')
    assert.equal(await tiles.count(), weapons.length)
    assert(await page.getByLabel('Subcategory', { exact: true }).isEnabled())
    assert(await page.getByLabel('Degree', { exact: true }).isEnabled())
    const degrees = await page.getByLabel('Degree', { exact: true }).locator('option').evaluateAll(options => options.map(option => option.value).filter(Boolean))
    if (degrees.length) {
      await page.getByLabel('Degree', { exact: true }).selectOption(degrees[0])
      assert.equal(await tiles.count(), weapons.filter(row => String(itemMetadata(row).degree) === degrees[0]).length)
    }
    await page.getByRole('button', { name: 'Reset Filters', exact: true }).click()
    // Exercise a real subcategory selection, then verify changing category
    // clears dependent subcategory/degree values.
    await page.getByLabel('Item Type', {exact:true}).selectOption('accessories')
    const necklaceCount = defaultRows.filter(row => itemMetadata(row).sort_type === 'Necklace').length
    if (necklaceCount) {
      await page.getByLabel('Subcategory', {exact:true}).selectOption('Necklace')
      assert.equal(await tiles.count(), necklaceCount)
      await page.getByLabel('Item Type', {exact:true}).selectOption('weapons')
      assert.equal(await page.getByLabel('Subcategory', {exact:true}).inputValue(), '')
      assert.equal(await page.getByLabel('Degree', {exact:true}).inputValue(), '')
    }
    await page.getByRole('button', {name:'Reset Filters',exact:true}).click()
    const geometry = await tiles.first().evaluate(tile => ({ height: tile.getBoundingClientRect().height, icon: tile.querySelector('.item-search-tile-icon').getBoundingClientRect().width, font: getComputedStyle(tile.querySelector('.item-detail-name')).fontSize }))
    assert.equal(Math.round(geometry.height), 64)
    assert.equal(Math.round(geometry.icon), 45)
    assert.equal(geometry.font, '13px')
    const tooltipTrigger = tiles.first()
    if (width === 390) await tooltipTrigger.tap()
    else await tooltipTrigger.focus()
    await page.locator('.item-tooltip.is-placed').waitFor()
    assert.equal(await page.getByRole('tooltip').locator('.item-tooltip-name').count(), 1)
    await page.keyboard.press('Escape')
    await page.getByRole('tooltip').waitFor({state:'detached'})
    // Inventory/storage buttons open the container dialog in place; the full
    // dialog flow is covered by item_dialog_browser_check.mjs.
    for (const [name, title] of [['Open Char Inventory', 'Inventory'], ['Open Char Storage', 'Storage']]) {
      const button = page.getByRole('button', { name, exact: true }).first()
      if (await button.count()) {
        const before = page.url()
        await button.click()
        const dialog = page.getByRole('dialog', { name: title, exact: true })
        await dialog.locator('.inventory-grid .item-slot').first().waitFor()
        assert.equal(page.url(), before)
        await dialog.getByRole('button', { name: 'Close', exact: true }).click()
        await dialog.waitFor({ state: 'detached' })
      }
    }
    await page.route('**/api/items?*', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"browser transport check"}' }))
    await page.getByRole('button', { name: 'Refresh items', exact: true }).click()
    await page.getByRole('alert').filter({hasText: 'Item Search could not be loaded.'}).waitFor()
    await page.unroute('**/api/items?*')
    await page.getByRole('button', { name: 'Retry', exact: true }).click()
    await page.waitForFunction(() => document.querySelector('.item-search-panel')?.getAttribute('aria-busy') === 'false')
    assert.equal(await page.getByRole('alert').count(), 0)
    await search.fill('a')
    await page.getByRole('heading', {name:'Item Search', exact:true}).click()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
    await page.screenshot({ path: join(directory, `items-${width}x${height}.png`) })
    const offersResponse = await page.request.get(`${origin}/api/economy/offers?limit=1`)
    assert.equal(offersResponse.status(), 200, 'Economy proxy has the same missing-route defect')
    assert.equal(offersResponse.headers()['cache-control'], 'no-store')
    const offers = await offersResponse.json()
    await page.goto(`${origin}/economy`)
    await page.waitForFunction(() => !document.body.innerText.includes('Loading offers…') && /\d+ (messages|offers)/.test(document.body.innerText))
    assert.equal(await page.getByRole('alert').count(), 0)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
    await page.screenshot({ path: join(directory, `economy-${width}x${height}.png`), fullPage: true })
    assert.deepEqual(errors, [])
    results.push({ width, height, itemTotal: first.total, offerTotal: offers.total, authenticatedProxy: true, queryAndCursor: true, completeGroups: true, categoryAndDegree: true, sourceNavigation: true, geometry, filterReset: true, itemTooltip: true, errorRecovery: true, overflow: false, browserErrors: 0 })
    await context.close()
  }
} catch (error) {
  const failedPage = browser.contexts().at(-1)?.pages().at(-1)
  if (failedPage) await failedPage.screenshot({path: join(directory, 'items-failure.png')}).catch(() => {})
  throw error
} finally {
  await browser.close()
}
writeFileSync(join(directory, 'search-results.json'), `${JSON.stringify(results, null, 2)}\n`)
console.log(JSON.stringify(results))
