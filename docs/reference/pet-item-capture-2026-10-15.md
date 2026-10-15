# Pick-pet item capture — 2026-10-15

The operator supplied `pet-item-capture-20261015T212731Z.json` as local runtime
evidence. Its SHA-256 is
`3e6462ac4fc778a6f9c6d40d2cd02744807474c83126fea8ecc9763f38f0db6e`.
The raw file contains game-session identifiers and is not committed.

## What was observed

- Probe 0.2.0 was armed with a Pick pet already summoned. The file ends by
  operator action 3.609 seconds later. It has one complete, untruncated,
  267-byte incoming `0x30C8` packet at 0.454 seconds. It has **no** incoming
  `0xB034` operation and no outgoing `0x7034` request.
- The `0x30C8` packet begins with the newly summoned pet ID and model 7493.
  The pet ID differs from the initial API snapshot. The packet has capacity 28
  and 10 occupied item entries. The following API snapshot has exactly the same
  slots: 0–8 and 16. The same items were present before resummoning; this is
  a pet-data refresh, not evidence of a new pickup.
- Reading the entries using the [RSBot pet handler](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Cos/CosDataResponse.cs),
  [inventory collection](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Inventory/InventoryItemCollection.cs)
  and [item parser](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Inventory/InventoryItem.cs)
  as a *candidate* format consumes all 267 bytes. It uses a four-byte zero
  rental type and two empty binding groups per equipment item. All 10 decoded
  slot, model, quantity and durability values match the subsequent phBot
  `get_pets()` rows, including seven model-10912 weapons, one model-11125
  staff and two stackable elixirs. This validates the observed boundaries for
  this packet and these item families; it does not validate other rental,
  binding, pet-family or game-server branches.
- The candidate equipment fields contain enhancement value 7 for the eight
  weapons, durability 93 for model 10912 and 39 for model 11125, and an
  eight-byte attribute value of 7,707,214,913 for the staff. The API confirms
  durability but has no enhancement, attribute or blue-option fields to
  independently compare. All candidate magic-option counts in this packet
  are zero. No blue-option encoding is verified here.
- The only later character-inventory change is an MP potion at slot 18. No
  pet-held item moves to the bag in this file, so same-instance comparison is
  unavailable.

## Decision and next evidence

Do not activate pet item enrichment or verified pet-pickup receipts from this
first file. A packet parser could decode this one simple snapshot, but it has no
observed blue options, no operation packet, no item-transfer comparison and no
nonzero rental/binding branch. The remaining runtime gate is a bounded capture
of an actual `0xB034` pet pickup and a pet-to-bag move, preferably including a
blue-option item. Compare the packet fields with the richer `get_inventory()`
row for the **same moved item**, while distinguishing it from duplicate models.
The item parser's byte order and variant branches remain hypotheses until those
observations agree. The independent source is a protocol lead, not a phBot API
guarantee.

## Second capture: confirmed pet-to-bag transfer

The operator also supplied `pet-item-capture-20261015T212929Z.json` (SHA-256
`298f46c0cbf7dfe0d17f95a6720a527fc6bb14443bf9d14cc18136be185c5aad`).
The raw file is not committed. It contains two outgoing `0x7034` requests and
two incoming `0xB034` responses, all untruncated, across 11.442 seconds. There
is no `0x30C8` pet snapshot and no pet pickup operation in this second capture.

The packet pairs have the same pet ID and slot transitions as the API snapshots:

| Time | Request/result subtype | Packet slots | Observed API change |
| --- | --- | --- | --- |
| 5.14/5.20 s | `0x10` | pet 12 → pet 19, quantity 1 | model 14650 disappears from pet slot 12 and appears at slot 19 |
| 11.14/11.20 s | `0x1A` | pet 19 → character bag 64 | pet slot 19 disappears; bag slot 64 gains model 14650 |

The independent [operation enum](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Inventory/InventoryOperation.cs)
and [handler](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Inventory/InventoryOperationResponse.cs)
agree with those move semantics. Both `0xB034` responses contain only result,
operation type, pet ID, slots and (for `0x10`) quantity; **neither response
contains an item-instance body**.

The moved item is Divine Weave, model 14650. Its pet API row had durability
243 and no plus/whites/blues. The newly occupied bag slot 64 has the same model
and durability, plus 5, six blue options and six white percentages, all 100.
An already equipped copy of model 14650 also had plus 5 and the same options,
but durability 65. The packet-linked slot transition identifies the moved copy;
model equality alone would have been ambiguous. The rich bag row demonstrates
the fields phBot can expose *after* transfer, but it does not show where those
bytes appeared while the item was still held by the pet.

This is enough to validate these two movement subtypes for the observed Pick
pet/session. It still cannot validate a parser for pet-held rare item blues:
the first `0x30C8` snapshot had no item with blues, and the second capture has
no initial item body or floor-to-pet receipt. The next useful capture must
include an incoming `0xB034` result with **subtype `0x11`**, which the cited
handler treats as floor-to-pet, for a newly picked up equipment item with blue
options. Then compare its pet slot and, if moved, the same bag copy. This
request is for operator-controlled test activity; the probe will not move items.

Probe 0.2.2 records counts by opcode and successful `0xB034` subtype even when
the payload buffer fills. It logs a warning if no candidate floor-to-pet
`0x11` operation was seen, since ordinary pet moves alone cannot verify rich
pet item data.

## Third capture: rich Pick-pet pickup bodies

The operator supplied `pet-item-capture-20261015T213224Z.json` (SHA-256
`73a87eaeea727ad2aeeef5933c1dcbe88d7cdb17d991d241516248e5bbb9c7b0`).
The raw file is not committed. It has two bag drop requests, their successful
`0xB034` subtype `0x07` results, and two subsequent successful subtype `0x11`
Pick-pet results. All six packets are complete. This is operator-provided data,
not an instruction to operate characters.

The 49-byte subtype `0x11` body names pet ID 4560936, slot 9, model 5573,
plus 0, variance 9743371, durability 58 and blue pairs `(54,5),(11,1)`.
The 81-byte body names the same pet, slot 10, model 14650, plus 5, variance
1073741823, durability 243 and six blue pairs `(88,160),(136,40),(76,6),
(208,6),(70,6),(202,6)`. Both decode fully with the observed zero-rental,
empty-binding equipment layout. The initial bag snapshot contains those
same two models and durability values, plus/white/blue details. Each is absent
from the bag and present at the named pet slot after its packet. API blue pair
order differs from wire order, but the pair multisets agree. The 30-bit
all-31 variance of model 14650 agrees with its six 100% white rolls. The
second capture's same-model equipped copy has durability 65, so the 243
match matters.

Plugin 1.9.41 now decodes this bounded subtype `0x11` equipment branch after
vSRO 1.188 detection. It waits for `get_pets()` to match pet ID, slot, model,
durability, quantity and available plus before attaching an `instance` to the
Pick-pet slot. It invalidates on removal, new pet snapshot, unsupported
operation, session/protocol reset or queue overflow. The confirmed subtype
`0x10` slot move moves its observation; subtype `0x1A` pet-to-bag removes it.
The existing Go item-detail resolver and UI consume the same instance shape
as character equipment. This implementation does not infer plus/blues for
items already in the pet before observation, other pet classes, rentals,
nonempty bindings, or unverified packet variants. This capture verifies
packet fields and state transitions, not a live run of plugin 1.9.41.
