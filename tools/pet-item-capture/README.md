# Pet item packet capture

A temporary **read-only** phBot plugin that collects the evidence needed to build a verified decoder for items held by pets. It never injects packets, contacts PhMon, reads chat or logs credentials.

Why it exists: phBot's `get_pets()` item rows contain only name, model, quantity, remaining, durability, expiration and servername. A pet-held rare therefore has no plus, white rolls or blues, even though the same item shows them once it is in the character's bag. PhMon's pet packet probe counts `0x30C8` / `0xB034` packets but decodes nothing, because the field layout has not been verified against real traffic. This capture produces the fixtures for that verification.

## Procedure

1. Copy `PetItemCapture.py` into the phBot `Plugins` directory of one test session and load it.
2. Select **Arm 10-minute capture**.
3. Within the window: dismiss and summon the pick pet once (this produces the full `0x30C8` pet inventory), and let it pick up at least three equipment drops. One item should be sealed (rare) and one should have blues if possible.
4. Move one of those items from the pet to the character bag in game, so the same instance appears in `get_inventory()` with its plus, whites and blues.
5. Select **Stop and write file**, or wait for the timeout. The log prints the file name.
6. Send the `pet-item-capture-<UTC>.json` file, then remove the plugin.

## What the file contains

* `packets`: up to 64 incoming `0x30C8` and `0xB034` packets with opcode, length, time and hex payload (each capped at 4 KiB; other opcodes are ignored).
* `snapshots`: `get_pets()` and `get_inventory()` item rows taken about once per second and after each pet packet, with only primitive values, at most 128 rows each. Unchanged interval samples are skipped.
* The file is capped at 2 MiB; if it would exceed that, snapshots are thinned. It is written next to the plugin and contains no character names.

## Next step

With one capture, compare each pet packet with the `get_pets()` rows around it and with the bag row of the moved item. Record the verified layout in `docs/item-instance-evidence.md`, then implement the decoder behind the existing `vsro_1188` protocol gate. Do not enable pet item decoding from this tool alone.
