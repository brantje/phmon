import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import {
  buildItemDetail,
  itemRecordFromActivityEvent,
  relatedItemDetail,
  itemTooltipPresentation,
  itemTooltipStatLines,
} from '../app/utils/itemDetailPopup.ts'

const hood = {
  plus: 7,
  quantity: 1,
  seal: 'Seal of Star',
  metadata: {
    name: 'Cruel Taesarin Hood',
    rare: true,
    sort_type: 'Garment',
    mounted_part: 'Head',
    degree: 7,
    type_ids: [1, 1, 1],
    required_level: 63,
    required_gender: 'Male',
    required_race: 'Chinese',
    icon_url: '/game-assets/icon/item/hood.png',
    reference_stats: { phy_def_pwr: { min: '40', max: '50' } },
  },
  instance_details: {
    stats: [
      {
        key: 'phy_def_pwr',
        label: 'Phy. def. pwr',
        value: '51.5',
        percent: 51,
      },
      { key: 'mag_def_pwr', label: 'Mag. def. pwr', value: '106', percent: 0 },
      { key: 'parry_ratio', label: 'Parry ratio', value: '23', percent: 32 },
    ],
    durability: { current: 62, maximum: 63 },
    percentages: [{ key: 'durability', label: 'Durability', value: 38 }],
    blues: [{ label: 'Int increase', value: '3', scale: 1, precision: 0 }],
  },
}

test('compact drop popup selects one observation without changing canonical drop facts', () => {
  const item = {
    model: 777,
    drop_unobserved: true,
    metadata: hood.metadata,
    related_item_observation: {
      association: 'nearby_same_model_observation',
      event_id: 'source-event',
      container: { type: 'inventory' },
      item: hood,
    },
  }
  const before = structuredClone(item)
  const displayed = itemTooltipPresentation(item)
  assert.equal(displayed.model.name, hood.metadata.name)
  assert.equal(displayed.model.plus, 7)
  assert.deepEqual(displayed.model.blues, ['Int 3 Increase'])
  assert.equal(displayed.model.catalogNote, '')
  assert.equal(displayed.caption, 'Observed item · pickup unverified')
  assert.match(displayed.sourceDescription, /source-event/)
  assert.deepEqual(item, before)
  assert.deepEqual(buildItemDetail(item).statsBeforeDurability, [])
  const direct = itemTooltipPresentation({
    ...hood,
    related_item_observation: item.related_item_observation,
  })
  assert.equal(direct.caption, '')
  assert.equal(direct.model.plus, 7)
})

test('an observation found after a connection gap is labelled and stays unverified', () => {
  const displayed = itemTooltipPresentation({
    drop_unobserved: true,
    related_item_observation: {
      association: 'gap_reconciled_same_model',
      event_id: 'gap-source',
      container: { type: 'inventory' },
      item: hood,
    },
  })
  assert.equal(displayed.model.plus, 7)
  assert.equal(
    displayed.caption,
    'Observed after a connection gap · pickup unverified',
  )
  assert.match(displayed.sourceDescription, /connection gap/)
  assert.match(displayed.sourceDescription, /gap-source/)
})

test('captured Gem Necklace renders its own rolls and blues in one compact item model', () => {
  const fixture = JSON.parse(
    readFileSync(
      new URL('./fixtures/gem-necklace-tooltip.json', import.meta.url),
      'utf8',
    ),
  )
  const displayed = itemTooltipPresentation(fixture.item)
  assert.equal(displayed.model.name, 'Gem Necklace')
  assert.equal(displayed.model.seal, 'Seal of Sun')
  assert.deepEqual(itemTooltipStatLines(displayed.model), [
    'Phy. absorption 27.5 (+48%)',
    'Mag. absorption 27.3 (+9%)',
  ])
  assert.deepEqual(displayed.model.blues, [
    'Freezing, frostbite 3% Reduce',
    'Electric shock 1% Reduce',
  ])
  assert.equal(displayed.caption, '')
})

test('basic pet item cannot acquire rolled stats from the catalog or another observation', () => {
  const displayed = itemTooltipPresentation({
    drop_unobserved: true,
    related_item_observation: {
      association: 'nearby_same_model_observation',
      event_id: 'pet-source',
      container: { type: 'pets' },
      item: {
        model: 11142,
        name: 'Lepus Soul Fiend',
        durability: 82,
        metadata: {
          type_ids: [3, 1, 6, 11],
          reference_stats: { mag_atk_pwr_min: { min: '2819', max: '2989' } },
        },
      },
    },
  })
  assert.deepEqual(displayed.model.statsBeforeDurability, [])
  assert.deepEqual(displayed.model.blues, [])
  assert.equal(displayed.model.durability, 'Durability 82')
  assert.equal(displayed.caption, 'Observed item · stats unavailable')
})

test('compact catalog-only items keep static ranges distinct from observed rolls', () => {
  const displayed = itemTooltipPresentation({
    metadata: {
      name: 'Catalog item',
      reference_stats: { phy_def_pwr: { min: '40', max: '50' } },
    },
  })
  assert.equal(displayed.caption, 'Catalog ranges · rolls unavailable')
  assert.deepEqual(displayed.model.statsBeforeDurability, [
    'Phy. def. pwr 40–50',
  ])
})

test('drop tooltip keeps a separate inventory observation with its stats and blues', () => {
  for (const kind of ['drop.item', 'drop.rare']) {
    for (const association of [
      'stored_unverified_drop_event_id',
      'nearby_same_model_observation',
      'gap_reconciled_same_model',
    ] as const) {
      const record = itemRecordFromActivityEvent({
        kind,
        item_model: 777,
        item_metadata: hood.metadata,
        payload: { model: 777 },
        related_item_observation: {
          event_id: '8d16ab39-10b8-4b26-80aa-a4be01e10f18',
          association,
          source: 'phbot.state_diff',
          occurred_at: '2026-10-15T16:16:38Z',
          container: { type: 'inventory', slot: 14 },
          item: hood,
        },
      })
      assert.ok(record)
      const drop = buildItemDetail(record)
      assert.deepEqual(drop.statsBeforeDurability, [])
      assert.deepEqual(drop.blues, [])
      assert.equal(drop.plus, null)
      const observed = relatedItemDetail(record)
      assert.ok(observed)
      assert.equal(observed.eventID, '8d16ab39-10b8-4b26-80aa-a4be01e10f18')
      assert.equal(observed.source, 'Inventory observation')
      assert.deepEqual(observed.model.statsBeforeDurability, [
        'Phy. def. pwr 51.5 (+51%)',
        'Mag. def. pwr 106 (+0%)',
      ])
      assert.deepEqual(observed.model.blues, ['Int 3 Increase'])
      assert.equal(observed.model.plus, 7)
      assert.equal(observed.model.durability, 'Durability 62/63 (+38%)')
    }
  }
})

test('unavailable, ambiguous or unsupported related observations never use current inventory', () => {
  assert.equal(relatedItemDetail(hood), null)
  assert.equal(
    relatedItemDetail({ related_item_observation: { item: hood } }),
    null,
  )
  assert.equal(
    relatedItemDetail({
      related_item_observation: {
        association: 'verified_pickup',
        event_id: 'some-event',
        item: hood,
      },
    }),
    null,
  )
  const pet = relatedItemDetail({
    related_item_observation: {
      association: 'nearby_same_model_observation',
      event_id: 'pet-event',
      container: { type: 'pets', id: '7' },
      item: hood,
    },
  })
  assert.equal(pet?.source, 'Pet inventory observation')
})

test('observed instance stats take precedence over catalog ranges', () => {
  const detail = buildItemDetail(hood)
  assert.equal(detail.name, 'Cruel Taesarin Hood')
  assert.equal(detail.plus, 7)
  assert.equal(detail.rare, true)
  assert.equal(detail.icon, '/game-assets/icon/item/hood.png')
  assert.deepEqual(detail.statsBeforeDurability, [
    'Phy. def. pwr 51.5 (+51%)',
    'Mag. def. pwr 106 (+0%)',
  ])
  assert.equal(detail.durability, 'Durability 62/63 (+38%)')
  assert.deepEqual(detail.statsAfterDurability, ['Parry ratio 23 (+32%)'])
  assert.equal(detail.catalogNote, '')
  assert.deepEqual(detail.blues, ['Int 3 Increase'])
  assert.deepEqual(
    detail.requirements.map((line) => line.text),
    ['Required level 63', 'Male', 'Chinese'],
  )
  assert.equal(detail.requirements[0]?.level, true)
})

test('Events drop item renders the reference observed suit details in order', () => {
  const record = itemRecordFromActivityEvent({
    kind: 'item.quantity_increased',
    item_model: 9001,
    item_metadata: {
      name: 'Shaman Suit',
      rare: true,
      sort_type: 'Garment',
      mounted_part: 'Shoulders',
      degree: 9,
      type_ids: [1, 1, 2],
      required_level: 81,
      required_gender: 'Male',
      required_race: 'Chinese',
      reference_stats: { phy_def_pwr: { min: '100', max: '222' } },
    },
    payload: {
      item: { model: 9001, servername: 'ITEM_CH_SUIT_09', plus: 5 },
      destination_container: { type: 'inventory', slot: 29 },
    },
    item_details: {
      seal: 'Seal of Sun',
      instance_details: {
        sources: ['phbot_api'],
        stats: [
          {
            key: 'phy_def_pwr',
            label: 'Phy. def. pwr',
            value: '221.8',
            percent: 100,
          },
          {
            key: 'mag_def_pwr',
            label: 'Mag. def. pwr',
            value: '473.5',
            percent: 100,
          },
          {
            key: 'parry_ratio',
            label: 'Parry ratio',
            value: '84',
            percent: 100,
          },
          {
            key: 'phy_reinforce',
            label: 'Phy. reinforce',
            value: '31.1%',
            percent: 100,
          },
          {
            key: 'mag_reinforce',
            label: 'Mag. reinforce',
            value: '66.3%',
            percent: 100,
          },
        ],
        durability: { current: 227, maximum: 93 },
        percentages: [{ key: 'durability', label: 'Durability', value: 100 }],
        blues: [
          { label: 'MP increase', raw_value: '600', scale: 1 },
          { label: 'Str increase', raw_value: '6', scale: 1 },
          { label: 'HP increase', raw_value: '600', scale: 1 },
          { label: 'Int increase', raw_value: '6', scale: 1 },
          { label: 'Lucky', raw_value: '6', scale: 1 },
          {
            label: 'Durability increase',
            raw_value: '160',
            scale: 1,
            unit: '%',
          },
          {
            label: 'Parry rate increase',
            raw_value: '40',
            scale: 1,
            unit: '%',
          },
          { label: 'Steady', raw_value: '6', scale: 1 },
        ],
      },
    },
  })
  assert.ok(record)
  const detail = buildItemDetail(record)
  assert.equal(detail.name, 'Shaman Suit')
  assert.equal(detail.plus, 5)
  assert.equal(detail.seal, 'Seal of Sun')
  assert.deepEqual(detail.classifications, [
    'Sort of item: Garment',
    'Mounting part: Shoulders',
    'Degree: 9 degrees',
  ])
  assert.deepEqual(detail.statsBeforeDurability, [
    'Phy. def. pwr 221.8 (+100%)',
    'Mag. def. pwr 473.5 (+100%)',
  ])
  assert.equal(detail.durability, 'Durability 227/93 (+100%)')
  assert.deepEqual(detail.statsAfterDurability, [
    'Parry ratio 84 (+100%)',
    'Phy. reinforce 31.1% (+100%)',
    'Mag. reinforce 66.3% (+100%)',
  ])
  assert.deepEqual(
    detail.requirements.map((line) => line.text),
    ['Required level 81', 'Male', 'Chinese'],
  )
  assert.deepEqual(detail.blues, [
    'MP 600 Increase',
    'Str 6 Increase',
    'HP 600 Increase',
    'Int 6 Increase',
    'Lucky(6Time/times)',
    'Durability 160% Increase',
    'Parry rate 40% Increase',
    'Steady(6Time/times)',
  ])
  assert.equal(detail.catalogNote, '')
})

test('percentage stat values keep their variance mark', () => {
  const detail = buildItemDetail({
    metadata: { name: 'Hood', type_ids: [1, 1, 1] },
    instance_details: {
      stats: [
        {
          key: 'phy_def_pwr',
          label: 'Phy. def. pwr',
          value: '51.5',
          percent: 51,
        },
        {
          key: 'mag_def_pwr',
          label: 'Mag. def. pwr',
          value: '106.0',
          percent: 0,
        },
        {
          key: 'phy_reinforce',
          label: 'Phy. reinforce',
          value: '12.3%',
          percent: 51,
        },
        {
          key: 'mag_reinforce',
          label: 'Mag. reinforce',
          value: '10.5% ~ 18.2%',
          percent: 6,
        },
      ],
      durability: { current: 62, maximum: 63 },
      percentages: [{ key: 'durability', label: 'Durability', value: 6 }],
    },
  })
  assert.deepEqual(detail.statsBeforeDurability, [
    'Phy. def. pwr 51.5 (+51%)',
    'Mag. def. pwr 106.0 (+0%)',
  ])
  assert.equal(detail.durability, 'Durability 62/63 (+6%)')
  assert.deepEqual(detail.statsAfterDurability, [
    'Phy. reinforce 12.3% (+51%)',
    'Mag. reinforce 10.5% ~ 18.2% (+6%)',
  ])
})

test('catalog ranges render only when no observed stats exist', () => {
  const detail = buildItemDetail({
    metadata: {
      name: 'Cruel Taesarin Hood',
      sort_type: 'Garment',
      degree: 1,
      reference_stats: {
        phy_def_pwr: { min: '40', max: '60' },
        parry_ratio: { min: '10', max: '10' },
        ignored: { min: 1, max: 2 },
      },
    },
  })
  assert.deepEqual(detail.statsBeforeDurability, [
    'Phy. def. pwr 40–60',
    'Parry ratio 10',
  ])
  assert.equal(
    detail.catalogNote,
    "Catalog ranges; this drop's rolled values were not observed.",
  )
  assert.deepEqual(detail.classifications, [
    'Sort of item: Garment',
    'Degree: 1 degree',
  ])
})

test('observed scalar stats suppress catalog ranges', () => {
  const detail = buildItemDetail({
    phy_def_pwr: 51,
    phy_def_pwr_percent: 12,
    metadata: {
      name: 'Armor',
      reference_stats: { mag_def_pwr: { min: '1', max: '2' } },
    },
  })
  assert.deepEqual(detail.statsBeforeDurability, ['Phy. def. pwr: 51 (+12%)'])
  assert.equal(detail.catalogNote, '')
})

test('rejects unsafe icon paths', () => {
  const detail = buildItemDetail({
    metadata: { name: 'Broken', icon_url: '/game-assets/../secret.png' },
  })
  assert.equal(detail.icon, '')
  assert.equal(detail.iconFallback, 'B')
})

test('activity events become item records only when an item is present', () => {
  const record = itemRecordFromActivityEvent({
    item_code: 'ITEM_HOOD',
    item_icon_url: '/game-assets/icon/item/hood.png',
    item_metadata: {
      name: 'Cruel Taesarin Hood',
      reference_stats: { phy_def_pwr: { min: '40', max: '60' } },
    },
    payload: { plus: 3, item: { quantity: 2 } },
  })
  assert.ok(record)
  const detail = buildItemDetail(record)
  assert.equal(detail.name, 'Cruel Taesarin Hood')
  assert.equal(detail.plus, 3)
  assert.equal(detail.quantity, 2)
  assert.equal(detail.icon, '/game-assets/icon/item/hood.png')
  assert.deepEqual(detail.statsBeforeDurability, ['Phy. def. pwr 40–60'])

  assert.equal(
    itemRecordFromActivityEvent({
      payload: { level: 72 },
    }),
    null,
  )
})

test('event item records prefer enriched immutable details over snapshot gaps', () => {
  const record = itemRecordFromActivityEvent({
    item_model: 847,
    payload: {
      item: { model: 847, servername: 'ITEM_TEST', quantity: 2 },
      packet_observation: { observation_id: 'session-5:sequence-9' },
    },
    item_details: {
      model: 847,
      servername: 'ITEM_TEST',
      quantity: 2,
      plus: 5,
      metadata: { name: 'Test Armor', type_ids: [3, 1, 2, 0] },
      instance_details: {
        source: 'phbot_api',
        status: 'partial',
        stats: [{ key: 'phy_def_pwr', label: 'Phy. def. pwr', value: '55' }],
      },
    },
  })
  assert.ok(record)
  const detail = buildItemDetail(record)
  assert.equal(detail.name, 'Test Armor')
  assert.equal(detail.plus, 5)
  assert.deepEqual(detail.statsBeforeDurability, ['Phy. def. pwr 55'])
  assert.equal(
    (record.instance as Record<string, unknown>).observation_id,
    'session-5:sequence-9',
  )
})

test('drop tooltip uses linked observed stats and blues, and hides unobserved catalog ranges', () => {
  const metadata = {
    name: 'Test Armor',
    type_ids: [3, 1, 2, 0],
    reference_stats: { phy_def_pwr: { min: '40', max: '60' } },
  }
  const unobserved = itemRecordFromActivityEvent({
    kind: 'drop.item',
    item_model: 777,
    item_metadata: metadata,
    payload: { model: 777 },
  })
  assert.ok(unobserved)
  const unknown = buildItemDetail(unobserved)
  assert.deepEqual(unknown.statsBeforeDurability, [])
  assert.match(unknown.catalogNote, /not observed/)

  const linked = itemRecordFromActivityEvent({
    kind: 'drop.item',
    item_model: 777,
    item_metadata: metadata,
    payload: {
      model: 777,
      item: { model: 777, plus: 3, servername: 'ITEM_TEST' },
      item_observation: { association: 'unique_model_inventory_gain' },
    },
    item_details: {
      instance_details: {
        sources: ['phbot_api'],
        stats: [{ key: 'phy_def_pwr', label: 'Phy. def. pwr', value: '55' }],
        blues: [
          { label: 'Int increase', raw_value: '3', scale: 1, precision: 0 },
        ],
      },
    },
  })
  assert.ok(linked)
  const observed = buildItemDetail(linked)
  assert.deepEqual(observed.statsBeforeDurability, ['Phy. def. pwr 55'])
  assert.deepEqual(observed.blues, ['Int 3 Increase'])
  assert.equal(observed.catalogNote, '')
  assert.equal(
    (linked.instance as Record<string, unknown>).association,
    'unique_model_inventory_gain',
  )

  const nearby = itemRecordFromActivityEvent({
    kind: 'drop.item',
    item_model: 777,
    item_metadata: metadata,
    payload: {
      model: 777,
      item: { model: 777, plus: 0, servername: 'ITEM_TEST' },
      item_observation: { association: 'unique_temporal_inventory_gain' },
    },
    item_details: linked,
  })
  assert.ok(nearby)
  assert.equal(
    (nearby.instance as Record<string, unknown>).association,
    'unique_temporal_inventory_gain',
  )
})

test('recipient-owned gain uses its observed instance instead of catalog ranges', () => {
  const record = itemRecordFromActivityEvent({
    kind: 'item.quantity_increased',
    item_model: 11840,
    item_metadata: {
      name: 'Hydra Thunder Gauntlet',
      type_ids: [3, 1, 2, 0],
      reference_stats: { phy_def_pwr: { min: '60', max: '90' } },
    },
    payload: {
      item: { model: 11840, servername: 'ITEM_EU_M_HEAVY_09_AA_B' },
      destination_container: { type: 'inventory', slot: 29 },
      acquisition_method: 'unknown',
    },
    item_details: {
      instance_details: {
        sources: ['phbot_api'],
        stats: [{ key: 'phy_def_pwr', label: 'Phy. def. pwr', value: '82' }],
        blues: [
          { label: 'Int increase', raw_value: '3', scale: 1, precision: 0 },
        ],
      },
    },
  })
  assert.ok(record)
  const detail = buildItemDetail(record)
  assert.deepEqual(detail.statsBeforeDurability, ['Phy. def. pwr 82'])
  assert.deepEqual(detail.blues, ['Int 3 Increase'])
  assert.equal(detail.catalogNote, '')
})

test('ambiguous owned gain hides another copy’s rolls and catalog ranges', () => {
  const record = itemRecordFromActivityEvent({
    kind: 'item.quantity_increased',
    item_model: 11840,
    item_metadata: {
      name: 'Hydra Thunder Gauntlet',
      reference_stats: { phy_def_pwr: { min: '60', max: '90' } },
    },
    payload: {
      item: { model: 11840, servername: 'ITEM_EU_M_HEAVY_09_AA_B' },
      item_instance_unobserved: true,
    },
  })
  assert.ok(record)
  const detail = buildItemDetail(record)
  assert.deepEqual(detail.statsBeforeDurability, [])
  assert.match(detail.catalogNote, /could not be identified/)
})

test('basic owned gain never shows catalog ranges as its rolled stats', () => {
  const record = itemRecordFromActivityEvent({
    kind: 'item.acquired',
    item_model: 11840,
    item_metadata: {
      name: 'Hydra Thunder Gauntlet',
      reference_stats: { phy_def_pwr: { min: '60', max: '90' } },
    },
    payload: { item: { model: 11840, servername: 'ITEM_EU_M_HEAVY_09_AA_B' } },
  })
  assert.ok(record)
  const detail = buildItemDetail(record)
  assert.deepEqual(detail.statsBeforeDurability, [])
  assert.match(detail.catalogNote, /not observed for this item gain/)
})
