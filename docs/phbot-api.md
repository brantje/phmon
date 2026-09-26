# phBot Plugin API Reference

> Source: [official phBot plugin API documentation](https://plugins.phbot.org/phbot-api)  
> Scraped and normalized: 2026-09-26  
> Scope: every API function/callback listed in the official **phBot API** index. Large example payloads are intentionally omitted; signatures, behavior, return semantics, and important caveats are retained.

The upstream documentation is uneven: some entries have detailed return schemas, while others only show a usage example. Where upstream provides no prose description or return value, this reference says so rather than inventing behavior.

## Client

Source: https://plugins.phbot.org/phbot-api/client

### `get_client()`

Returns information about the attached game client.

**Returns:** `None` or a dictionary containing:

- `window` — client window handle.
- `pid` — client process ID.
- `path` — client path.
- `running` — whether the process is currently running; phBot checks the process state each time this function is called.

## Guild

Source: https://plugins.phbot.org/phbot-api/guild

### `get_guild()`

Returns all guild members.

**Returns:** a dictionary keyed by guild-member ID, or `None` when the character is not in game.

### `get_guild_union()`

Returns the guilds in the current guild union.

**Returns:** a dictionary keyed by union ID, or `None` when the character is not in game.

## Events / plugin callbacks

Source: https://plugins.phbot.org/phbot-api/events

Only define callbacks that the plugin actually needs.

### `finished()`

Called while Python is being unloaded and phBot is exiting. Use it to close files and release plugin-owned resources.

### `connected()`

Called when phBot connects to the game server.

### `disconnected()`

Called when phBot disconnects from the server. The callback may fire multiple times for one disconnect.

### `handle_joymax(opcode, data)`

Called for every packet received from the game server.

### `handle_silkroad(opcode, data)`

Called for every packet received from the game client.

### `joined_game()`

Called after the user selects a character successfully. Character data has not been loaded yet at this point.

### `teleported()`

Called after a teleport and also immediately after `joined_game()`.

### `event_loop()`

Called every 500 ms.

### `character_listing(args)`

Called when the character list is received.

`args` is a list of character-name strings. A character currently being deleted has a leading `*` in its name.

### `handle_chat(t, player, msg)`

Called for every received chat message.

- `t` is the chat type supplied by the server.
- `player` may be `None` when the message is not private.
- `msg` is the received message.

### `handle_event(t, data)`

Called for specific phBot events. `data` is always passed as a string; cast it when numeric data is required.

Documented event types:

| Constant | Value | `data` |
| --- | ---: | --- |
| `EVENT_UNIQUE_SPAWN` | 0 | Monster name |
| `EVENT_HUNTER_SPAWN` | 1 | Player name; includes traders |
| `EVENT_THIEF_SPAWN` | 2 | Player name |
| `EVENT_TRANSPORT_DIED` | 3 | Transport ID; includes horses |
| `EVENT_PLAYER_ATTACKING` | 4 | Player name |
| `EVENT_RARE_DROP` | 5 | Item model; equippable items only |
| `EVENT_ITEM_DROP` | 6 | Item model; equippable items only |
| `EVENT_DIED` | 7 | Empty string |
| `EVENT_ALCHEMY_FINISHED` | 8 | Empty string |
| `EVENT_GM_SPAWNED` | 9 | Player name |
| `EVENT_LEVEL_UP` | 10 | New level |

## Players

Source: https://plugins.phbot.org/phbot-api/players

### `get_players()`

**Upstream status: disabled.**

When available, this API represents nearby players.

**Returns:** `None` or a dictionary keyed by player ID. The documented payload includes fields such as name, guild, grant name, equipped items, coordinates, and dead/alive state.

## Party

Source: https://plugins.phbot.org/phbot-api/party

### `get_party()`

Returns all party members.

**Returns:** `None` or a dictionary keyed by the member's party ID.

Important notes:

- The result may be empty.
- `player_id` remains `0` until that player has spawned nearby.
- `hp_percent` and `mp_percent` use Joymax's 0–10 scale rather than 0–100.

## NPC

Source: https://plugins.phbot.org/phbot-api/npc

### `get_npcs()`

Returns nearby NPCs and teleporters.

**Returns:** `None` or a dictionary keyed by NPC ID. Documented fields include name, server name, model, region, and coordinates.

### `get_npc_goods(model)`

Returns the shop goods associated with an NPC model.

**Returns:** `None` or a nested dictionary of goods organized by page and slot.

## Character

Source: https://plugins.phbot.org/phbot-api/character

### `get_character_data()`

Returns statistics and identity data for the current character.

**Returns:** `None` or a dictionary. The documented data includes server, name, model, guild, job name, position, HP/MP, level, death state, gold, EXP/SP, job EXP, player/account IDs, locale, and EXP ratio. The dictionary may be empty.

### `get_position()`

Returns the current character position.

**Returns:** `None` or a dictionary with `region`, `x`, `y`, and `z`.

### `get_active_skills()`

Returns the currently active skills.

**Returns:** `None` or a dictionary keyed by skill ID.

### `get_skills()`

Returns all skills known to the player.

**Returns:** `None` or a dictionary keyed by skill ID. Documented fields include name, server name, level, mastery, cast time, cooldown, duration, active state, and whether the skill can currently be cast.

### `get_mastery()`

Returns the player's skill masteries.

**Returns:** `None` or a dictionary keyed by mastery ID, including the mastery name, level, and SP information.

## Academy

Source: https://plugins.phbot.org/phbot-api/academy

### `get_academy()`

Returns academy membership information.

**Returns:** `None` or academy-member data. The upstream example is dictionary-shaped and includes member online state, member type, coordinates, level, name, plus the academy ID.

## Inventory

Source: https://plugins.phbot.org/phbot-api/inventory

### `get_inventory()`

Returns the current character inventory.

**Returns:** `None` or an object containing inventory `size`, `gold`, and an `items` list. Empty slots are `None`.

### `get_storage()`

Returns the current character's personal storage.

If the character has not opened storage, the result contains no items.

**Returns:** `None` or an object containing storage size and an item list.

### `get_guild_storage()`

Returns the current character's guild storage.

If guild storage has not been opened, the result contains no items.

**Returns:** `None` or an object containing storage size and an item list.

### `get_job_pouch()`

Returns the current character's job-pouch items.

**Returns:** `None` or an object containing pouch size and an item list.

### `sort_inventory()`

Sorts the player's inventory.

Sorting runs on a separate thread, and the API does not notify the plugin when sorting has completed.

**Returns:** `True` or `False`.

### `use_return_scroll()`

Uses a return scroll from the player's inventory.

**Returns:** `True` or `False`.

### `reverse_return(type, name)`

Uses a reverse-return scroll.

Documented `type` values:

| Type | Destination |
| ---: | --- |
| 0 | Last return-scroll location |
| 1 | Last death location |
| 2 | Party member |
| 3 | Named location |

For types that use a target, `name` is the party-member or location name.

## Pets

Source: https://plugins.phbot.org/phbot-api/pets

### `get_pets()`

Returns the character's currently summoned pets.

**Returns:** `None` or a dictionary keyed by pet ID. The documented payload can include name, server name, model, type, HP, mounted state, and pet inventory.

Documented pet types are `none`, `fellow`, `horse`, `pick`, `transport`, and `wolf`. The string `none` indicates that phBot could not identify the pet because of an error.

## Monsters

Source: https://plugins.phbot.org/phbot-api/monsters

### `get_monsters()`

Returns all nearby monsters.

**Returns:** `None` or a dictionary keyed by monster ID. The documented data includes name, server name, model, type, region, position, HP/max HP, and attack state. The dictionary may be empty.

## Encoding

Source: https://plugins.phbot.org/phbot-api/encoding

### `get_encoding()`

Returns the text encoding used by the current Silkroad locale. This is useful when parsing packets containing locale-dependent text such as chat.

The docs specifically note that iSRO, SilkroadR, and cSRO SilkroadR use `utf-16le`; vSRO may use `utf-16le` or `cp1251`.

**Returns:** an encoding name string or `None` before a locale is selected. Other documented encodings include `gb18030`, `cp1251`, `big5`, `sjis`, and `euc-kr`.

## Locale

Source: https://plugins.phbot.org/phbot-api/locale

### `get_locale()`

Returns the current game locale as an unsigned byte.

Documented values:

| Value | Locale |
| ---: | --- |
| 2 | kSRO |
| 18 | iSRO |
| 65 | SilkroadR |
| 22 | vSRO, including official, 1.188, 1.193, 1.274, Black Rogue, and thSRO variants |
| 23 | Official vSRO 2 Job |
| 34 | cSRO SilkroadR private servers |
| 9 | ECSRO |
| 46 | jSRO GameCom |
| 52 | cSRO SilkroadR official |
| 54 | DIGEAM |
| 56 | TRSRO |
| 59 | ruSRO |

## Config

Source: https://plugins.phbot.org/phbot-api/config

### `get_config_path()`

Returns the current player's JSON configuration-file path.

The upstream docs warn that changes made directly to this file may later be overwritten by phBot.

**Returns:** a string path, or `None` when the player is not in game.

### `get_config_dir()`

Returns the phBot `Config` directory with a trailing forward slash.

**Returns:** string.

### `get_log_dir()`

Returns the phBot `Log` directory with a trailing forward slash.

**Returns:** string.

## Botting

Source: https://plugins.phbot.org/phbot-api/botting

### `start_bot()`

Starts botting.

**Returns:** `True` on success, otherwise `False`.

### `stop_bot()`

Stops botting.

**Returns:** `True` on success, otherwise `False`.

### `start_trace(name)`

Starts tracing the named player.

**Returns:** `True` on success, otherwise `False`.

### `stop_trace()`

Stops tracing.

**Returns:** `True` on success, otherwise `False`.

### `start_trade()`

Starts auto-trade.

**Returns:** `True` on success, otherwise `False`.

### `stop_trade()`

Stops auto-trade.

**Returns:** `True` on success, otherwise `False`.

## Taxi

Source: https://plugins.phbot.org/phbot-api/taxi

### `get_taxi()`

Returns players associated with the character's taxi party.

**Returns:** `None` or a dictionary keyed by player name.

Important fields/caveats documented upstream:

- `remaining` is remaining time in seconds.
- `party` tells whether the player is also in your party.
- `player_id` may be `0` if that player has not been seen in game.
- Some fields are absent for taxi members who are not in the party.

## Packet Injection

Source: https://plugins.phbot.org/phbot-api/packet-injection

### `inject_joymax(opcode, data, encrypted)`

Sends a packet to Joymax/the game server.

Parameters documented upstream:

- `opcode` — unsigned short.
- `data` — bytes.
- `encrypted` — boolean.

### `inject_silkroad(opcode, data, encrypted)`

Sends a packet to Silkroad/the game client.

Uses the same parameter types as `inject_joymax()`.

For complex packet construction, the upstream docs point to ProjectHax's `pySilkroadSecurity` stream helper.

## Log

Source: https://plugins.phbot.org/phbot-api/log

### `log(text)`

Appends text to phBot's main log.

**Returns:** `None`.

## Game Data

Source: https://plugins.phbot.org/phbot-api/game-data

### `get_item(id)`

Looks up item game data by numeric item ID.

**Returns:** `None` or an item-data object. Documented fields include server name, display name, `tid1`–`tid3`, cash-item flag, max stack, and level.

### `get_item_string(str)`

Looks up item game data by item server-name string.

**Returns:** `None` or the same item-data shape returned by `get_item()`.

### `get_monster(id)`

Looks up character/monster game data by numeric ID.

**Returns:** `None` or a game-data object. The documented example includes server name, name, level, and HP.

### `get_monster_string(str)`

Looks up character/monster game data by server-name string.

**Returns:** `None` or the same data shape returned by `get_monster()`.

### `get_skill(id)`

Looks up skill game data by numeric skill ID.

**Returns:** `None` or a skill-data object. Documented fields include server name, duration, SP, cooldown, MP, mastery, name, cast time, and level.

### `get_zone_name(region)`

Returns the in-game zone name for a region ID.

## Teleport

Source: https://plugins.phbot.org/phbot-api/teleport

### `get_teleport_data(source, destination)`

Returns the data needed to use a teleporter.

`source` and `destination` may be NPC names, location names, or server names.

**Returns:** `None` when no route data is found, otherwise a tuple. The upstream docs state that the tuple's second value is the teleport code needed after selecting the NPC/teleporter. This API itself does not send packets.

## Training Area

Source: https://plugins.phbot.org/phbot-api/training-area

### `set_training_position(region, x, y, z)`

Sets coordinates for the active training area. If no training area is active, nothing is changed.

- When `region` is `0`, phBot calculates the region for non-cave areas.
- Cave areas require an explicit region, for example from `get_character_data()`.
- Coordinates are floats.

**Returns:** `True` or `False` depending on whether the position was set.

### `set_training_script(path)`

Sets the script path for the active training area.

Passing an empty path resets the training-area script.

**Returns:** `True` when changed; otherwise `None`.

### `set_training_radius(radius)`

Changes the active training-area radius.

**Returns:** `True` or `False` depending on whether the radius was set.

### `get_training_area()`

Returns information about the enabled training area.

**Returns:** `None` or a dictionary containing coordinates, region, script path, training radius, and pick radius.

### `set_training_area(name)`

Switches the active training area by name.

**Returns:** `True` if changed, otherwise `False`.

## Command Line Arguments

Source: https://plugins.phbot.org/phbot-api/command-line-arguments

### `get_command_line_args()`

Returns all command-line arguments passed to phBot.

**Returns:** a list of strings, or `None` if the arguments could not be retrieved.

## Movement

Source: https://plugins.phbot.org/phbot-api/movement

### `move_to(x, y, z)`

Moves the character to the specified coordinates.

Important behavior:

- `z` may be `0`.
- The call is non-blocking and does not wait for the destination to be reached.
- Movement on transports/pets is supported.

**Returns:** `None`.

### `move_to_region(region, x, y, z)`

Moves using an explicit region plus coordinates.

The upstream page provides the signature and usage but no additional prose description.

**Returns:** `None`.

## Quests

Source: https://plugins.phbot.org/phbot-api/quests

### `get_quests()`

Returns all active quests.

**Returns:** a dictionary of active quests, or `None` when the character is not in game. Documented fields include quest type, name, NPC IDs, server name, objective-complete state, and completed state.

## Drops

Source: https://plugins.phbot.org/phbot-api/drops

### `get_drops()`

Returns nearby items that are pickable according to the current pick-filter settings.

**Returns:** a dictionary keyed by the pick ID used to pick up the item, or `None` when the character is not in game. Documented fields include item identity, region/coordinates, `can_pick`, blue-item state, and plus value.

## Paths

Source: https://plugins.phbot.org/phbot-api/paths

Path generation APIs are rate-limited by phBot to one call every five seconds.

### `generate_path(x, y)`

Generates a walking path to the requested coordinates.

**Returns:**

- `None` if no path could be found.
- `False` if five seconds have not elapsed since the last call or the character is not in game.
- Otherwise, a list of coordinate tuples.

Cave path tuples include an extra region field at position 0. Teleporting is not supported by this path form.

### `generate_script(region, x, y, z)`

Generates a phBot script path to the destination, including teleport and wait commands when required.

**Returns:**

- `None` if no path could be found.
- `False` if five seconds have not elapsed since the last call or the character is not in game.
- Otherwise, a list of script-command strings.

## Script

Source: https://plugins.phbot.org/phbot-api/script

### `start_script(str)`

Starts executing the supplied phBot script text in the background.

### `stop_script()`

Stops the currently executing script.

The upstream page does not document return values for these functions.

## Notifications

Source: https://plugins.phbot.org/phbot-api/notifications

### `create_notification(string)`

Adds a notification to the phBot notification log.

**Returns:** `True` when added, otherwise `False`.

### `create_notification_item(string, id)`

Adds a notification with an item icon. `id` is the item's model ID.

**Returns:** `True` when added, otherwise `False`.

> Upstream's usage example appears to call `create_notification(..., id)` even though the documented API heading is `create_notification_item(string, id)`; this reference preserves the documented function name.

## Alchemy

Source: https://plugins.phbot.org/phbot-api/alchemy

### `start_alchemy()`

Starts alchemy.

**Returns:** `True` or `False` indicating whether the operation succeeded.

### `stop_alchemy()`

Stops alchemy.

**Returns:** `True` or `False` indicating whether the operation succeeded.

### `reset_alchemy()`

Clears the alchemy queue.

**Returns:** `True` or `False` indicating whether the operation succeeded.

### `add_alchemy(dict)`

Adds an alchemy operation to the queue.

The documented dictionary example contains fields such as `type`, inventory `slot`, target `stop` plus, success/failure delays, powder slot, astral/steady/immortal/lucky slots, maximum attempts, destroyed-item stop behavior, and skip-failure behavior.

**Returns:** `True` or `False` indicating whether the operation succeeded.

### `alchemy_update(slot, success, plus)`

Plugin callback emitted to all plugins after an elixir is used on an item.

- `slot` — item slot.
- `success` — success state.
- `plus` — resulting plus value.

The upstream page documents this as an event callback rather than a function the plugin calls.

## Misc

Source: https://plugins.phbot.org/phbot-api/misc

### `get_version()`

Returns the phBot version as a string.

### `select_character(name)`

Selects a character by name.

**Returns:** `None`.

### `set_profile(name)`

Changes the active profile for the character.

**Returns:** `True` when the profile changed, otherwise `False`.

### `get_profile()`

Returns the current profile name.

**Returns:** a string, which may be empty for the default profile, or `None` when not logged in.

### `disconnect()`

Disconnects from the server without changing relog settings.

**Returns:** `None`.

### `show_notification(title, message)`

Shows a tray notification when phBot is minimized.

**Returns:** `True` when the arguments are valid, otherwise `False`.

### `play_wav(path)`

Plays a WAV audio file.

**Returns:** `None`.

### `minimize()`

Minimizes the phBot window.

**Returns:** `None`.

### `unminimize()`

Restores/unminimizes the phBot window.

**Returns:** `None`.

> The upstream heading says `unminize()`, but its usage example uses `unminimize()`; this reference uses the usage spelling.

## Coverage checklist

The official API index currently contains these 30 sections, all represented above:

Client, Guild, Events, Players, Party, NPC, Character, Academy, Inventory, Pets, Monsters, Encoding, Locale, Config, Botting, Taxi, Packet Injection, Log, Game Data, Teleport, Training Area, Command Line Arguments, Movement, Quests, Drops, Paths, Script, Notifications, Alchemy, and Misc.
