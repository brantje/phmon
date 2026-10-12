#!/usr/bin/env node

// Read-only browser acceptance against an isolated fixture instance. Never
// supplies monitoring fixtures or sends commands to a production instance.
import assert from 'node:assert/strict'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const origin = process.env.PHMON_IDENTITY_WEB_URL || 'http://127.0.0.1:3105'
const target = new URL(origin)
assert(['127.0.0.1', 'localhost'].includes(target.hostname), 'Use an isolated loopback instance')
assert(process.env.PHMON_IDENTITY_FIXTURE_SERVER, 'Set PHMON_IDENTITY_FIXTURE_SERVER for synthetic data')
assert(process.env.OPERATOR_ACCESS_SECRET, 'Set the isolated instance operator secret')
const scope = process.env.PHMON_IDENTITY_FIXTURE_SERVER
const directory = process.env.PHMON_IDENTITY_SCREENSHOTS || '/tmp/phmon-identity-browser'
mkdirSync(directory, { recursive: true })
const { chromium } = await import(process.env.PHMON_PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] })
const evidence = []
try {
  for (const [width, height] of [[1440, 1000], [1280, 800], [390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height } })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    const login = await page.request.post(`${origin}/api/auth/login`, {
      headers: { origin }, data: { secret: process.env.OPERATOR_ACCESS_SECRET },
    })
    assert.equal(login.status(), 200, 'Fixture login failed')
    const listURL = `${origin}/players?server=${encodeURIComponent(scope)}`
    await page.goto(listURL)
    await page.locator('.player-table tbody tr').first().waitFor()
    const initial = await page.request.get(`${origin}/api/players?server=${encodeURIComponent(scope)}`)
    assert.equal(initial.status(), 200)
    const data = await initial.json()
    assert.equal(data.total, 4, 'Expected grouped fixtures with own character hidden')
    assert.equal(await page.getByLabel('Hide own characters').isChecked(), true)
    const mapped = data.players.find(player => player.display_identity?.player_name === 'Magik')
    assert(mapped, 'Missing fixture Magik')
    assert.equal(mapped.display_identity.job_alias, 'xDont')
    assert.equal(mapped.display_identity.related_record_ids.length, 2)
    const normal = data.players.find(player => player.observed_name === 'NamedOnly')
    const alias = data.players.find(player => player.observed_name === 'AliasOnly')
    const unknown = data.players.find(player => player.observed_name === 'Unclassified')
    assert.equal(normal.display_identity.job_alias, null)
    assert.equal(alias.display_identity.player_name, null)
    assert.equal(unknown.display_identity.player_name, null)
    assert.equal(unknown.display_identity.job_alias, null)
    assert.equal(await page.locator('.player-table tbody tr').count(), 4)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, 'Page overflow')
    await page.screenshot({ path: join(directory, `players-${width}x${height}.png`), fullPage: true })
    await page.locator('input[type=search]').fill('xDont')
    await page.waitForFunction(() => document.querySelectorAll('.player-table tbody tr').length === 1 && location.search.includes('q=xDont'))
    const row = page.locator('.player-table tbody tr').first()
    assert((await row.innerText()).includes('Magik'))
    assert((await row.innerText()).includes('Inferred · ph-proxy'))
    await row.focus()
    await page.keyboard.press('Enter')
    await page.waitForURL(`**/players/${mapped.id}?**`)
    await page.locator('.player-visible-gear .equipment-slot').first().waitFor()
    assert.equal(await page.locator('.player-visible-gear .equipment-slot').count(), 7)
    assert((await page.locator('.player-page').innerText()).includes('Enrichment sources'))
    await page.screenshot({ path: join(directory, `player-detail-${width}x${height}.png`), fullPage: true })
    const aliasID = mapped.display_identity.related_record_ids.find(id => id !== mapped.id)
    await page.goto(`${origin}/players/${aliasID}?back=${encodeURIComponent(`/players?server=${scope}&q=xDont`)}`)
    await page.locator('.player-visible-gear .equipment-slot').first().waitFor()
    assert.equal(await page.locator('.player-visible-gear .equipment-slot').count(), 7, 'Alias URL lost canonical equipment')
    await page.getByRole('link', { name: 'Back to Players' }).click()
    await page.locator('.player-table tbody tr').first().waitFor()
    await page.locator('input[type=search]').fill('NoSuchFixturePlayer')
    await page.getByText('No observed players match these filters.').waitFor()
    // A failed read and subsequent successful read must recover visibly.
    await page.route('**/api/players?*', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"service unavailable"}' }))
    await page.locator('input[type=search]').fill('FailureFixture')
    await page.getByText('The player registry is unavailable.').waitFor()
    await page.unroute('**/api/players?*')
    await page.locator('input[type=search]').fill('Magik')
    await page.locator('.player-table tbody tr').first().waitFor()
    await page.getByLabel('Hide own characters').uncheck()
    await page.waitForURL('**hide_own=0**')
    await page.locator('input[type=search]').fill('OwnCharacter')
    await page.waitForFunction(() => document.querySelector('.player-table tbody')?.textContent.includes('OwnCharacter'))
    if (!(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))) {
      await page.screenshot({ path: join(directory, `overflow-${width}x${height}.png`), fullPage: true })
      console.log(JSON.stringify(await page.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth, elements: [...document.querySelectorAll('body *')].filter(el => {const r=el.getBoundingClientRect();return r.right>innerWidth+1 && r.left>=0}).map(el => ({ tag:el.tagName, class:el.className, width:el.getBoundingClientRect().width, right:el.getBoundingClientRect().right })).slice(0,20) }))))
    }
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, 'Recovered page overflow')
    assert.deepEqual(errors, [], 'Browser errors')
    evidence.push({ width, height, groupedRows: data.total, gearItems: 7, aliasDetail: true, ownFilter: true, keyboardNavigation: true, emptyErrorRecovery: true, pageOverflow: false })
    await context.close()
  }
} finally {
  await browser.close()
}
writeFileSync(join(directory, 'results.json'), `${JSON.stringify(evidence, null, 2)}\n`)
console.log(JSON.stringify(evidence))
