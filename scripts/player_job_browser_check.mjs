#!/usr/bin/env node
// Read-only acceptance against a synthetic fixture instance. Never seeds
// production monitoring, changes proxy settings or sends bot commands.
import assert from 'node:assert/strict'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const origin = process.env.PHMON_JOB_WEB_URL || 'http://127.0.0.1:3107'
assert(['127.0.0.1', 'localhost'].includes(new URL(origin).hostname), 'Use an isolated loopback instance')
const scope = process.env.PHMON_JOB_FIXTURE_SERVER
assert(scope && process.env.OPERATOR_ACCESS_SECRET, 'Set the fixture server and isolated operator secret')
const directory = process.env.PHMON_JOB_SCREENSHOTS || '/tmp/phmon-job-browser'
mkdirSync(directory, { recursive: true })
const { chromium } = await import(process.env.PHMON_PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] })
const results = []
try {
  for (const [width, height] of [[1440, 1000], [1280, 800], [390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height } })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    const login = await page.request.post(`${origin}/api/auth/login`, { headers: { origin }, data: { secret: process.env.OPERATOR_ACCESS_SECRET } })
    assert.equal(login.status(), 200)
    const listURL = `${origin}/players?server=${encodeURIComponent(scope)}`
    await page.goto(listURL)
    await page.locator('.player-table tbody tr').first().waitFor()
    const response = await page.request.get(`${origin}/api/players?server=${encodeURIComponent(scope)}`)
    assert.equal(response.status(), 200)
    const data = await response.json()
    assert.equal(data.total, 6)
    const row = name => page.locator('.player-table tbody tr').filter({ has: page.getByText(name, { exact: true }) })
    const expected = { NormalTrader: 'Trader', ProxyZero: 'None', UnknownCode: 'Unknown (255)', NativeZero: 'None', NativeHunter: 'Hunter', AliasAttributed: 'Unknown' }
    for (const [name, job] of Object.entries(expected)) {
      const cell = row(name).locator('td').nth(5)
      assert((await cell.innerText()).startsWith(job), `${name}: wrong job display`)
      assert.equal(await row(name).locator('td').nth(7).innerText(), 'No', 'Affiliation changed suit state')
    }
    assert.equal((await row('NormalTrader').locator('td').nth(5).innerText()).trim(), 'Trader')
    assert.equal(await row('NativeZero').locator('td').nth(6).innerText(), '0')
    assert.equal(await row('NativeHunter').locator('td').nth(6).innerText(), '4')
    assert.equal(await row('AliasAttributed').locator('td').nth(6).innerText(), 'Unknown')
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
    await page.screenshot({ path: join(directory, `jobs-${width}x${height}.png`), fullPage: true })
    await page.getByLabel('Job', { exact: true }).selectOption('255')
    await page.waitForFunction(() => document.querySelectorAll('.player-table tbody tr').length === 1 && document.querySelector('.player-table tbody')?.textContent.includes('UnknownCode'))
    assert.equal(await page.getByLabel('Job', { exact: true }).inputValue(), '255')
    for (const [name, job] of Object.entries(expected)) {
      const player = data.players.find(player => player.observed_name === name)
      assert(player)
      const detailResponse = await page.request.get(`${origin}/api/players/${player.id}`)
      assert.equal(detailResponse.status(), 200)
      const detail = await detailResponse.json()
      assert.equal(detail.enrichment.field_evidence.job.reason, 'unverified_reference_mapping')
      await page.goto(`${origin}/players/${player.id}`)
      await page.getByRole('heading', { name: 'ph-proxy enrichment', exact: true }).waitFor()
      assert.equal((await page.locator('.player-page').innerText()).includes('unverified_reference_mapping'), true)
      const overview = page.locator('.player-card').first()
      const fact = label => overview.locator('.player-facts > div').filter({ has: page.locator('dt').getByText(label, { exact: true }) }).locator('dd')
      assert((await fact('Job').innerText()).startsWith(job), `${name}: wrong detail job display`)
      const itemFact = page.locator('.player-facts > div').filter({ has: page.getByText('Worn job items', { exact: true }) }).locator('dd')
      assert.equal(await itemFact.innerText(), 'None observed')
      if (name === 'NativeZero') assert.equal(await fact('Job Level').innerText(), '0')
      if (name === 'AliasAttributed') {
        assert.equal(await fact('Job').innerText(), 'Unknown')
        assert((await page.locator('.player-page').innerText()).includes('Inferred'))
      }
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
      if (name === 'NormalTrader' || name === 'AliasAttributed') await page.screenshot({ path: join(directory, `${name}-${width}x${height}.png`), fullPage: true })
    }
    assert.deepEqual(errors, [])
    results.push({ width, height, normalWithoutSuit: true, zero: true, nativePrecedence: true, aliasAttribution: true, unknownCodeFilter: true, confidence: true, pageOverflow: false, browserErrors: errors })
    await context.close()
  }
} finally {
  await browser.close()
}
writeFileSync(join(directory, 'results.json'), `${JSON.stringify(results, null, 2)}\n`)
console.log(JSON.stringify(results))
