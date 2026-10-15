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
  [Ability-pet body](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Cos/Ability.cs),
  [inventory collection](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Inventory/InventoryItemCollection.cs)
  and [item parser](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Inventory/InventoryItem.cs)
  as a *candidate* format consumes all 267 bytes. It uses a four-byte zero
  rental type and two empty binding groups per equipment item. All 10 decoded
  slot, model, quantity and durability values match the subsequent phBot
  `get_pets()` rows, including seven model-10912 weapons, one model-11125
  staff and two stackable elixirs. This validates the observed boundaries for
  this packet and these item families; it does not validate other rental,
  binding, pet-family or game-server branches.
- The complete Pick-pet header is object ID, model ID, current/max HP,
  settings, a length-prefixed name, capacity and occupied count. After the
  item entries are a four-byte owner object ID and a one-byte inventory slot.
  A strict re-read of this packet identified the final `0x2e` byte as that
  slot; without it the earlier analysis left one byte unconsumed. The cited
  Ability-pet parser corroborates this trailer, and the revised reader consumes
  all 267 bytes exactly.
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

## Fourth capture: rich items already held at summon

`pet-item-capture-20261015T214035Z.json` (SHA-256
`42f3f5255e5c623d18c26d5131cdfd59219532a1e5e70b23458389185dddac35`)
contains one complete 417-byte incoming `0x30C8` Pick-pet snapshot and an API
sample 29 ms later. The raw file is not committed. The operator had two rich
items already in the pet before summon. A strict reader consumes the full
header, all twelve item records, the owner ID and the final inventory-slot
byte. All twelve slot, model, quantity and durability values agree with
`get_pets()`; ten are equipment and two are simple stackable elixirs. The
shared equipment reader used by the verified `0xB034` subtype `0x11` branch
also consumes both rich item bodies without a special case:

| Pet slot | Item | Packet plus | Variance | Durability | Blue ID/value pairs |
| --- | --- | ---: | ---: | ---: | --- |
| 25 | Gold Earring, model 14811 | 5 | 1023 | 0 | `(6,3),(12,3),(30,10),(24,10),(18,10),(36,10),(42,10)` |
| 26 | Magic Weave, model 14631 | 5 | 1073741823 | 75 | `(81,60),(129,20),(6,3),(12,3),(54,6)` |

The packet has no `0x1A` move and the initial bag sample has neither model,
so this file alone cannot independently compare these values with the *same*
physical item's richer `get_inventory()` row. The fifth capture below supplies
that cross-check for Magic Weave. Gold Earring's blue pairs remain structurally
decoded but lack an independent same-item bag comparison.

## Fifth capture: same Magic Weave copy reaches the bag

`pet-item-capture-20261015T214407Z.json` (SHA-256
`d0f2a3d4f74fff0d5fbc332499f6dbe9cec1078c4bacb2b97dcb426fbe5c657a`)
contains one complete `0x7034` request and its successful `0xB034` subtype
`0x1A` result. Both identify pet ID 4584763, source slot 26 and bag slot 72.
The pet API row at slot 26 before the move is Magic Weave model 14631 with
durability 75; after the move that pet slot is empty and bag slot 72 holds
model 14631, durability 75, plus 5. This is a packet-linked physical transfer,
not a same-model/time inference. The raw file is not committed.

The bag's five `(blue ID, value)` pairs are exactly the `0x30C8` snapshot's
five pairs as a multiset: `(81,60),(129,20),(6,3),(12,3),(54,6)`.
Packet ordering and phBot dictionary ordering differ. The snapshot's variance
is `1073741823` (`0x3fffffff`), whose six five-bit fields are all 31; the
bag API reports six white percentages of 100. This independently verifies
the snapshot's plus, variance/white-roll and blue-option bytes for that item.

Plugin 1.9.42 now decodes the strict, observed Pick-pet `0x30C8` branch as
well as `0xB034` subtype `0x11`. It requires a complete API roster match for
pet ID, model, capacity, occupied count, item slots, models, quantities and
durabilities, then uses the existing pet-slot `instance` detail path. Any
unsupported item family, nonzero rental, nonempty binding, malformed field
or trailing byte rejects the full snapshot. Simple stackable alchemy elixirs
are parsed only to advance to later equipment; no synthetic instance is
created for them. Actual Windows/phBot execution of plugin 1.9.42 remains
unverified; offline capture decoding and fixture tests are not a live rollout.

## 2026-10-16 live Kalypso restart: packet/API capacity mismatch

The operator restarted Kalypso with plugin 1.9.42. The server recorded a fresh
session and one `0x30C8` packet of 245 bytes, but zero enriched pet items.
The pet API reported 56 positions with eight equipment items, all in slots
0–16. Both earlier complete `0x30C8` captures advertise 28 positions. The
245-byte live packet is also exactly the size obtained by removing the two
11-byte elixir entries from the earlier 267-byte packet. Its bytes were not
retained by the live diagnostic, so its capacity value is inferred from those
captures, not read directly. The 1.9.42 parser required packet capacity to
equal the full API list length and would reject a 28-versus-56 layout.

Plugin 1.9.43 accepts either equal lengths or an API list twice the packet
capacity. It still requires the packet's occupied count and all covered
slots, models, quantities and durabilities to match, and rejects any occupied
API slot beyond the packet's capacity. The item-enrichment diagnostic now
reports the last snapshot mismatch reason. Actual 1.9.43 live attachment
remains to be verified after the operator installs it and resummons the pet.
