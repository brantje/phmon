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
      { key: 'parry_ratio', label: 'Parry rate', value: '23', percent: 32 },
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
  assert.deepEqual(detail.statsAfterDurability, ['Parry rate 23 (+32%)'])
  assert.equal(detail.catalogNote, '')
  assert.deepEqual(detail.blues, ['Int 3 Increase'])
  assert.deepEqual(
    detail.requirements.map((line) => line.text),
    ['Required level 63', 'Male', 'Chinese'],
  )
})

test('pet packet enhancement and resolved blues render without pet API detail fields', () => {
  const detail = buildItemDetail({
    model: 14631,
    servername: 'ITEM_EU_W_CLOTHES_03_FA_C_RARE',
    durability: 75,
    metadata: {
      name: 'Magic Weave',
      type_ids: [3, 1, 9, 6],
    },
    instance: {
      availability: 'observed',
      source: 'vsro_1188_packet',
      plus: 5,
    },
    instance_details: {
      durability: { current: 75, maximum: null },
      percentages: [{ key: 'durability', label: 'Durability', value: 100 }],
      blues: [
        { label: 'Durability increase', raw_value: '60', scale: 1, unit: '%' },
        { label: 'Parry rate increase', raw_value: '20', scale: 1, unit: '%' },
        { label: 'Int increase', raw_value: '3', scale: 1 },
        { label: 'Str increase', raw_value: '3', scale: 1 },
        { label: 'Steady', raw_value: '6', scale: 1 },
      ],
    },
  })
  assert.equal(detail.plus, 5)
  assert.equal(detail.durability, 'Durability 75 (+100%)')
  assert.deepEqual(detail.blues, [
    'Durability 60% Increase',
    'Parry rate 20% Increase',
    'Int 3 Increase',
    'Str 3 Increase',
    'Steady(6Time/times)',
  ])
  assert.equal(detail.catalogNote, '')
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
            label: 'Parry rate',
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
    'Parry rate 84 (+100%)',
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
    'Parry rate 10',
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
  assert.deepEqual(detail.statsBeforeDurability, ['Phy. def. pwr 51 (+12%)'])
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

test('weapon tooltip follows the in-game section order with blue roll percentages', () => {
  const detail = buildItemDetail({
    plus: 7,
    metadata: {
      name: 'Soul Fiend',
      seal: 'Seal of Star',
      rare: true,
      type_ids: [3, 1, 6, 11],
      sort_type: 'Weapon',
      item_group: 'Twohand staff',
      degree: 10,
      required_level: 90,
      required_race: 'European',
      attack_distance: '0.6',
      max_magic_options: 9,
    },
    instance_details: {
      durability: { current: 291, maximum: 291 },
      percentages: [{ key: 'durability', label: 'Durability', value: 100 }],
      stats: [
        { key: 'critical_ratio', label: 'Critical', value: '2', percent: 100 },
        {
          key: 'mag_atk_pwr',
          label: 'Mag. atk. pwr',
          value: '3357 ~ 3966',
          percent: 100,
        },
        { key: 'hit_ratio', label: 'Attack rate', value: '329', percent: 100 },
      ],
      blues: [
        { id: '209', label: 'Lucky', raw_value: '2', unit: ' uses', scale: 1 },
        {
          id: '101',
          label: 'Attack rate increase',
          raw_value: '40',
          unit: '%',
          scale: 1,
          percent: 63,
        },
        {
          id: '71',
          label: 'Str increase',
          raw_value: '6',
          scale: 1,
          percent: 100,
        },
      ],
    },
  })
  assert.deepEqual(itemTooltipStatLines(detail), [
    'Mag. atk. pwr 3357 ~ 3966 (+100%)',
    'Durability 291/291 (+100%)',
    'Attack distance 0.6',
    'Attack rate 329 (+100%)',
    'Critical 2 (+100%)',
  ])
  assert.deepEqual(detail.classifications, [
    'Sort of item: Twohand staff',
    'Degree: 10 degrees',
  ])
  assert.deepEqual(
    detail.requirements.map((line) => line.text),
    ['Required level 90', 'European', 'Max no. of magic options: 9'],
  )
  assert.deepEqual(detail.blues, [
    'Str 6 Increase (+100%)',
    'Attack rate 40% Increase (+63%)',
    'Lucky(2Time/times)',
  ])
  assert.deepEqual(detail.notes, ['Able to use Advanced elixir.'])
})

test('robe lists mastery requirements and keeps armor stat order', () => {
  const detail = buildItemDetail({
    metadata: {
      name: 'Divine Robe',
      type_ids: [3, 1, 9, 3],
      item_group: 'Robe',
      mounted_part: 'Chest',
      required_gender: 'Female',
      required_race: 'European',
      mastery_requirements: [
        { mastery: 'Wizard', level: 92 },
        { mastery: 'Cleric', level: 92 },
      ],
    },
    instance_details: {
      stats: [
        { key: 'phy_reinforce', label: 'Phy. reinforce', value: '33.0%' },
        { key: 'parry_ratio', label: 'Parry ratio', value: '85' },
        { key: 'mag_def_pwr', label: 'Mag. def. pwr', value: '446.8' },
        { key: 'phy_def_pwr', label: 'Phy. def. pwr', value: '228.6' },
      ],
    },
  })
  assert.deepEqual(itemTooltipStatLines(detail), [
    'Phy. def. pwr 228.6',
    'Mag. def. pwr 446.8',
    'Parry rate 85',
    'Phy. reinforce 33.0%',
  ])
  assert.deepEqual(
    detail.requirements.map((line) => line.text),
    [
      'Mastery level: Wizard 92',
      'Mastery level: Cleric 92',
      'Female',
      'European',
    ],
  )
})

test('packet rolls skip stats the item lacks and use tooltip labels', () => {
  const detail = buildItemDetail({
    metadata: {
      type_ids: [3, 1, 6, 11],
      reference_stats: {
        phy_atk_pwr_min: { min: '0', max: '0' },
        phy_atk_pwr_max: { min: '0', max: '0' },
        mag_atk_pwr_min: { min: '2580', max: '2737' },
        mag_atk_pwr_max: { min: '3153', max: '3346' },
      },
    },
    instance_details: {
      percentages: [
        { key: 'phy_atk_pwr', label: 'Phy. atk. pwr', value: 100 },
        { key: 'mag_atk_pwr', label: 'Mag. atk. pwr', value: 90 },
        { key: 'hit_ratio', label: 'Attack rating', value: 41 },
      ],
    },
  })
  assert.deepEqual(detail.statsAfterDurability, [
    'Mag. atk. pwr (+90%)',
    'Attack rate (+41%)',
  ])
})

test('ring resist blues share one phrasing', () => {
  const blue = (id: string, label: string) => ({
    id,
    label,
    raw_value: '20',
    unit: '%',
    scale: 1,
    percent: 100,
  })
  const detail = buildItemDetail({
    metadata: { type_ids: [3, 1, 12, 3] },
    instance_details: {
      blues: [
        blue('191', 'Zombie duration reduction'),
        blue('179', 'Burn duration reduction'),
        blue('185', 'Poison duration reduction'),
      ],
    },
  })
  assert.deepEqual(detail.blues, [
    'Burn 20% Reduce (+100%)',
    'Poisoning 20% Reduce (+100%)',
    'Zombie 20% Reduce (+100%)',
  ])
})

test('advanced elixir note only on elixir equipment without an applied elixir', () => {
  const note = (item: Record<string, unknown>) => buildItemDetail(item).notes
  assert.deepEqual(note({ metadata: { type_ids: [3, 1, 4, 1] } }), [
    'Able to use Advanced elixir.',
  ])
  assert.deepEqual(note({ metadata: { type_ids: [3, 1, 13, 2] } }), [])
  assert.deepEqual(note({ metadata: { type_ids: [3, 1, 7, 1] } }), [])
  assert.deepEqual(
    note({
      metadata: { type_ids: [3, 1, 6, 11] },
      instance_details: {
        blues: [
          {
            id: '1',
            label: 'Advanced elixir is in effect',
            raw_value: '2',
            scale: 1,
          },
        ],
      },
    }),
    [],
  )
})

test('catalog reinforce ranges are shown as percentages', () => {
  const detail = buildItemDetail({
    metadata: {
      type_ids: [3, 1, 9, 3],
      reference_stats: {
        phy_reinforce: { min: '299', max: '330' },
        mag_reinforce: { min: '703', max: '703' },
      },
    },
  })
  assert.deepEqual(itemTooltipStatLines(detail), [
    'Phy. reinforce 29.9%–33.0%',
    'Mag. reinforce 70.3%',
  ])
})
