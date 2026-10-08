// Explicit local fixture acceptance. Install Playwright outside the project or
// expose it through NODE_PATH. Equipment fixtures do not verify game packets.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

async function main() {
  const base = process.env.SMOKE_WEB_URL
  assert.ok(['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  const fixture = JSON.parse(fs.readFileSync(process.env.PLAYER_BROWSER_FIXTURE, 'utf8'))
  assert.equal(fixture.fixture, true)
  const output = process.env.PLAYER_BROWSER_ARTIFACTS || '/tmp/phmon-player-browser/artifacts'
  fs.mkdirSync(output, { recursive: true })
  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] })
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const page = await context.newPage()
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))
  const url = (query = {}) => `${base}/players?${new URLSearchParams({ server: fixture.server, ...query })}`
  const rows = () => page.locator('.player-table-panel tbody tr')
  const waitList = async () => { await page.waitForTimeout(100); await page.locator('.player-table-panel[aria-busy=false]').waitFor() }
  const list = async (query = {}) => { await page.goto(url(query)); await waitList() }
  await page.goto(url())
  await page.getByLabel('Operator access secret').fill(process.env.OPERATOR_ACCESS_SECRET)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await waitList()
  assert.equal(await rows().count(), 25)
  assert.ok(await page.locator('a[href="/players"]').first().evaluate((e) => e.className.includes('active')))
  const checks = [
    { q: 'FixturePlayer00' }, { guild: 'Fixture Guild' }, { min_level: '110' },
    { max_level: '104' }, { job: 'hunter' }, { job: 'trader' }, { job: 'thief' },
    { job: 'unknown' }, { seen: '24h' }, { seen: '7d' }, { seen: '30d' },
    { identity: 'unresolved' }, { equipment: 'partial' }, { equipment: 'complete' },
    { equipment: 'unavailable' }, { guild: 'Fixture Guild', min_level: '100', max_level: '110', job: 'hunter', equipment: 'partial' },
    { seen: 'custom', from: new Date(Date.now()-86400000).toISOString().slice(0,10), to: new Date().toISOString().slice(0,10) },
  ]
  for (const query of checks) {
    await list(query)
    assert.ok(await rows().count() > 0, `filter returned no fixture rows: ${JSON.stringify(query)}`)
    assert.equal(new URL(page.url()).searchParams.get('server'), fixture.server)
  }
  for (const query of [{ job: 'none' }, { identity: 'resolved' }, { q: 'NoSuchFixture' }, { seen: 'custom', to: '2000-01-01' }]) {
    await list(query)
    assert.equal(await rows().count(), 0, `empty filter ${JSON.stringify(query)} at ${page.url()}`)
    await page.getByText('No matching players', { exact: true }).waitFor()
  }
  await list({ seen: '1h' })
  const recent = await context.request.get(`${base}/api/players?${new URLSearchParams({ server: fixture.server, seen: '1h' })}`)
  assert.equal(recent.status(), 200)
  assert.equal(await rows().count(), Math.min(25, (await recent.json()).total))
  for (const [label,key] of [['Name','name'],['Level','level'],['Guild','guild'],['Job','job'],['Last seen','last_seen']]) {
    await list()
    await page.getByRole('button', { name: `${label} ↕`, exact: true }).click()
    await waitList()
    const firstDirection = new URL(page.url()).searchParams.get('direction') || 'desc'
    if (key !== 'last_seen') assert.equal(new URL(page.url()).searchParams.get('sort'),key)
    await page.getByRole('button', { name: `${label} ↕`, exact: true }).click()
    await waitList()
    assert.notEqual(new URL(page.url()).searchParams.get('direction') || 'desc', firstDirection)
  }
  await list()
  await page.getByLabel('Name or alias', { exact: true }).fill('FixturePlayer00')
  await page.waitForURL(/q=FixturePlayer00/)
  await waitList()
  assert.equal(await rows().count(), 1)
  await page.reload(); await waitList()
  assert.equal(await page.getByLabel('Name or alias', { exact: true }).inputValue(), 'FixturePlayer00')
  await page.getByLabel('Guild', { exact: true }).fill('Missing Guild')
  await page.waitForURL(/guild=Missing/); await waitList()
  assert.equal(await rows().count(), 0)
  await page.goBack(); await waitList(); assert.equal(await rows().count(), 1)
  await page.goForward(); await waitList(); assert.equal(await rows().count(), 0)
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click()
  await page.waitForURL(/server=all$/); await waitList()
  await list({ limit: '10' })
  await page.getByRole('button', { name: 'Name ↕', exact: true }).click()
  await page.waitForURL(/sort=name/); await waitList()
  const firstRows = await rows().allTextContents()
  await page.getByRole('button', { name: 'Next', exact: true }).click()
  await page.waitForURL(/cursor=/); await waitList()
  assert.equal(await rows().count(), 10)
  await page.reload(); await waitList()
  await page.getByRole('button', { name: 'Previous', exact: true }).click()
  await page.waitForURL(/page=1/); await waitList()
  assert.deepEqual(await rows().allTextContents(), firstRows)
  for (const [width,height] of [[1440,1000],[1280,800],[390,844]]) {
    await page.setViewportSize({ width,height }); await list()
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth<=innerWidth+1), `list overflow ${width}`)
    await page.screenshot({ path:path.join(output,`player-list-${width}.png`) })
  }
  await page.setViewportSize({ width:1440,height:1000 })
  const profile = `${base}/players/${fixture.profile_id}?return_to=${encodeURIComponent(url().replace(base,''))}`
  await page.goto(profile)
  await page.getByRole('heading',{ name:'Character details',exact:true }).waitFor()
  await page.getByRole('button',{ name:'Unknown chest',exact:true }).waitFor()
  await page.getByRole('button',{ name:'Empty slot 8',exact:true }).waitFor()
  await page.locator('.player-raw-stats summary').first().click()
  await page.locator('.player-raw-stats dd').filter({hasText:'18446744073709551615'}).waitFor()
  const item = page.locator('.player-equipment .item-detail-popup .item-slot').first()
  if (await item.count()) { await item.focus(); await page.locator('[role=tooltip]').waitFor(); await page.keyboard.press('Escape') }
  const reason = 'Clearly labeled browser simulator decision'
  const save = async () => {
    await page.getByLabel('Evidence or correction reason').fill(reason)
    await page.getByLabel('I reviewed the identities and confirm this decision.').check()
    await page.getByRole('button',{name:'Save decision',exact:true}).click()
    await page.locator('.player-review-form').waitFor({state:'hidden'})
  }
  await page.getByRole('button',{name:'Classify observed alias',exact:true}).click()
  await page.getByLabel('Identity type').selectOption('normal'); await save()
  await page.getByRole('heading',{name:'FixturePlayer00',exact:true}).waitFor()
  await page.getByRole('button',{name:'Link another identity',exact:true}).click()
  await page.getByLabel('Other player ID').fill(fixture.target_id)
  await page.getByRole('button',{name:'Load player',exact:true}).click()
  await page.locator('.player-review-form p').filter({hasText:'FixturePlayer02'}).waitFor()
  await save()
  await page.getByRole('button',{name:'Unlink',exact:true}).waitFor()
  await page.goto(`${base}/players/${fixture.target_id}`)
  await page.getByText('This identity is associated with a canonical player.',{exact:false}).waitFor()
  await page.getByRole('button',{name:'Unlink',exact:true}).click();await save()
  await page.getByRole('heading',{name:'FixturePlayer02',exact:true}).waitFor()
  await page.goto(`${base}/players/${fixture.candidate_profile_id}`)
  await page.getByRole('button',{name:'Review match',exact:true}).click()
  await page.getByLabel('Decision',{exact:true}).selectOption('reject')
  await page.locator('.player-review-form p').filter({hasText:'FixturePlayer04'}).waitFor()
  await save()
  await page.getByText('rejected · alias-conflict-v1',{exact:true}).waitFor()
  for (const [width,height] of [[1440,1000],[1280,800],[390,844]]) {
    await page.setViewportSize({width,height}); await page.goto(profile)
    await page.getByRole('heading',{name:'Character details',exact:true}).waitFor()
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth<=innerWidth+1),`profile overflow ${width}`)
    await page.screenshot({path:path.join(output,`player-profile-${width}.png`)})
  }
  await page.setViewportSize({width:1280,height:800})
  await page.goto(`${base}/players/00000000-0000-4000-8000-000000000001`)
  await page.getByText('Player not found',{exact:true}).waitFor()
  await page.route('**/api/players?*', route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":"fixture database unavailable"}'}))
  await page.goto(url())
  await page.getByText('Player registry unavailable',{exact:true}).waitFor()
  await page.unroute('**/api/players?*')
  await page.getByRole('button',{name:'Retry',exact:true}).click();await waitList()
  await page.goto(`${base}/map`)
  await page.locator('.map-page').waitFor().catch(()=>page.waitForTimeout(1000))
  await page.screenshot({path:path.join(output,'map-regression-1280.png')})
  assert.deepEqual(errors,[])
  console.log(JSON.stringify({result:'PASS',fixture:true,filters:checks.length+5,sorts:5,viewports:[1440,1280,390],manual_review:true,error_recovery:true,errors}))
  await browser.close()
}
main().catch(e=>{console.error(e);process.exit(1)})
