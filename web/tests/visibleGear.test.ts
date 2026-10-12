import assert from 'node:assert/strict'
import test from 'node:test'
import { buildItemDetail } from '../app/utils/itemDetailPopup.ts'
import {
  visibleGearItemRecord,
  visibleGearLabel,
} from '../app/utils/visibleGear.ts'

test('visible gear uses numeric reference when metadata name is absent', () => {
  const item = visibleGearItemRecord({
    ref_item_id: 424242,
    opt_level: null,
    slot: null,
  })
  assert.equal(item.servername, 'Item #424242')
  const detail = buildItemDetail(item)
  assert.equal(detail.name, 'Item #424242')
  assert.equal(detail.plus, null)
  assert.match(detail.catalogNote, /Visible equipment references/)
})

test('visible gear label keeps unknown slot implicit', () => {
  assert.equal(visibleGearLabel({ ref_item_id: 1, slot: null }), 'Visible item')
  assert.equal(
    visibleGearLabel({ ref_item_id: 1, slot: 7 }),
    'Visible item (game slot 7)',
  )
})

test('visible equipment only suppresses catalog reference ranges', () => {
  const detail = buildItemDetail({
    model: 100,
    visible_equipment_only: true,
    metadata: {
      name: 'Catalog Sword',
      reference_stats: { phy_atk_pwr: { min: '10', max: '20' } },
    },
  })
  assert.equal(detail.statsBeforeDurability.length, 0)
  assert.match(detail.catalogNote, /Rolled stats, blues, durability/)
})
