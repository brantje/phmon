# Pet item packet capture

A temporary **read-only** phBot plugin that collects the evidence needed to build a verified decoder for items held by pets. It never injects packets, contacts PhMon, reads chat or logs credentials.

Why it exists: phBot's public `get_pets()` example shows pet item slots with model, name, quantity, plus and durability, but does not document white rolls, blue options or full combat stats. The observed phBot 20.1.2 Pick-pet rows for this project had no rich item fields. `get_item(model)` supplies static game data, not a particular item's rolls. PhMon 1.9.43 decodes the verified Pick-pet equipment branches of `0x30C8` and `0xB034`. Other pet classes and variant item bodies still need evidence. This tool can collect bounded fixtures for those cases.

## Procedure

1. Copy `PetItemCapture.py` into the phBot `Plugins` directory of one test session and load it.
2. Select **Arm 10-minute capture**.
3. Within the window: dismiss and summon the Pick pet once to seek a full `0x30C8` pet-data packet, and let it pick up at least one equipment drop **after arming**. Prefer a sealed item with blue options. A successful incoming `0xB034` subtype `0x11` is the candidate floor-to-pet receipt; ordinary pet-slot or pet-to-bag moves do not carry the new item's detailed bytes. Do this only in an operator-authorized test session.
4. Move one of those items from the pet to the character bag in game, so the same instance appears in `get_inventory()` with its plus, whites and blues. An outgoing `0x7034` request may bracket the incoming `0xB034` result when the game client performs the move; a clientless session may not supply that callback.
5. Select **Stop and write file**, or wait for the timeout. Disconnecting or unloading the plugin also writes the current capture. The log prints the file name.
6. Send the `pet-item-capture-<UTC>.json` file, then remove the plugin.

## Pet level / HP capture (probe 0.3.0)

To learn where each pet class reports level and HP (phBot's `get_pets()` gives only an absolute `hp`, no level or max HP):

1. Arm the probe, then summon **each pet class you have, one at a time** (attack/fellow, wolf, horse/transport, pick), waiting a few seconds after each summon so its `0x30C8` header arrives.
2. For an attack/fellow/wolf pet, let it take damage or heal, and if possible gain a level or experience, so HP and level updates appear. Note the in-game level and HP/max HP values you see for each pet (write them down with the time) so the bytes can be matched.
3. Stop and send the file.

The file now keeps every primitive field phBot returns per pet (`hp`, `model`, `name`, `mounted`, and any level-like field if the runtime has one) and records any server packet whose first four bytes are a currently summoned pet's ID (6 per packet shape, where a shape is opcode + length + bytes 4..6, at most 64 shapes, so frequent HP updates `0x3057` cannot crowd out a rarer type) plus bare counts of all other opcodes. Nothing is decoded or enabled from this evidence yet.

### Hunger (HGP) search (probe 0.6.0)

The HGP value is only known to be in the `0x30C8` summon header. To find a live update packet, **arm the probe before summoning the wolf** so the header sets a baseline. For the rest of the capture the probe also keeps (6 per opcode, up to 256 bytes) any server packet that contains the pet's ID at any offset after the first four bytes, or a 16-bit value within 600 below / 5 above the summon HGP. Entries carry `scan_reasons` (for example `hgp_window_at_6`). Write down the in-game HGP at a few times so a candidate can be matched. Hits can be false positives.

## What the file contains

* `packets`: up to 64 incoming `0x30C8` and `0xB034` packets and outgoing `0x7034` item-move requests with direction, opcode, length, time and hex payload (each capped at 32 KiB; other opcodes are ignored). A `truncated` marker identifies an incomplete packet. `packet_counts` also records successful `0xB034` subtypes even if the packet buffer fills.
* `snapshots`: `get_pets()` and `get_inventory()` item rows taken about once per second and after each pet packet, with only primitive values, at most 128 rows each. Unchanged interval samples are skipped.
* The file is capped at 2 MiB; if necessary snapshots are thinned and late packets omitted with an explicit counter. It is written next to the plugin. Raw packet data can contain pet names or other personal game data, so review it before sharing. The probe does not intentionally collect chat, account credentials or character names.

## Next step

Compare each pet packet with the `get_pets()` rows around it and with the bag row of the moved item; use packet direction and timestamps to identify the move. The probe logs a warning when no candidate `0xB034` subtype `0x11` pickup was seen. Record newly verified layouts in `docs/item-instance-evidence.md` before extending the decoder behind the existing `vsro_1188` protocol gate. Another bot's parser is a lead, not validation for the installed phBot/game-server version. Do not enable pet item decoding from this tool alone.
