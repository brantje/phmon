# Operator-authorized item tooltip investigation

Date: 2026-09-27. Scope: understand item-instance collection and tooltip math for
Slice 4. The operator explicitly allowed inspection/decompilation of the supplied
phMonitor client, overriding the earlier project restriction for this task.

Artifact: operator-local `Downloads/phMonitor-v0.5.0.exe`, 77,774,336 bytes,
SHA-256 `6768f32f4615e9503449e24b147236a83b8ed97b3febb7f0c7e05b263f9415ae`.
Go build information identifies Go 1.25.2, module `sro-monitor`, revision
`201322617a925edb158a81a944dee0d20ad0feba` with local modifications. That build
identity does not establish the running reference server's version or source.

Static inspection found readable embedded Python and JavaScript, so native-code
decompilation was unnecessary for the relevant paths. The executable was not run.
No extracted source, executable, or reference asset catalog is added to PhMon or
used at runtime. This document records observations and independently expressed
mathematical behavior, not copied implementation.

## Collection findings

The inspected inventory path calls phBot's `get_inventory()` and normalizes each
returned item. It categorizes several potential field names for attributes and
magic options, including integer-keyed dictionaries. It is not waiting for a new
item-stat packet to build these tooltip lists. The same normalizer appears twice
in the embedded resources with identical content.

Reproducible byte offsets in this exact artifact:

| Inspected symbol | File offset | Relevant behavior |
| --- | ---: | --- |
| `_extract_tooltip_lists` | 35958208 | Reads option/attribute fields from API items; broadly flattens values. |
| `_normalize_item_entry` | 35960467 | Uses those fields when building an inventory item. |
| `normalize_inventory` | 35972615 | Obtains the inventory through the phBot getter. |
| `attributeMapping` | 30538462 | Interprets numeric attribute IDs according to inferred item family. |
| `humanMagicLabel` | 30536185 | Interprets dataset option codes and raw values. |
| `applyPlusStepToValue` | 30553495 | Applies reference enhancement increments. |
| `applyDisplayedBonusToValue` | 30554208 | Interpolates between reference values using an attribute percentage. |
| `statValueWithPlusAndBonus` | 30555244 | Combines enhancement, interpolation, rounding and parry modifiers. |
| `resolveMaxDurability` | 30546227 | Applies white and blue durability inputs in stages. |

The public [phBot inventory documentation](https://plugins.phbot.org/phbot-api/inventory)
shows basic item fields only. It is an example contract, not proof that the installed
runtime cannot expose additional fields. Before this investigation, the live PhMon
database already contained `api_fields.blues` on 65 items, but all retained values
were empty dictionaries. That cannot prove the original API dictionaries were empty:
PhMon's sanitizer discarded every non-string dictionary key.

## Reference mathematics and limitations

The following describes the inspected application, not a verified Silkroad formula:

- The normal tooltip path accepts percentages from API attributes, rather than
  calculating them from raw variance. Numeric attribute IDs are mapped by item family.
  The mapping uses name heuristics, including different rules for clothing and other
  armor. These rules need checking against live API data and PhMon's typed metadata.
- A scalar stat starts with a static base plus enhancement level times an increment.
  The reference rounds this intermediate result to one decimal; attack values are
  floored instead. Advanced elixir can contribute to effective enhancement.
- A displayed percentage contributes `percentage / 100 * (upper_reference - base)`.
  It does not independently multiply the whole resulting stat. Missing/nonpositive
  upper references cause the percentage contribution to be omitted.
- Final parry is floored; attacks, critical and blocking use integer rounding;
  other combat values use one decimal. Parry's blue percentage is applied after
  flooring the white result, then floored again.
- Maximum durability interpolates its white range and floors it, applies a blue
  durability percentage and floors again, then applies a maximum-durability penalty
  and floors again. The reference also clamps maximum upward to current durability;
  PhMon must not adopt that as verified game behavior.
- Option IDs resolve through a static ID-to-code catalog. Known codes such as
  `MATTR_INT`, `MATTR_MP`, `MATTR_SOLID` and `MATTR_ER` determine labels and units.
  The inspected labels use raw values (respectively integer increase, integer
  increase, uses, percentage). The generic unknown-code name fallback is not a
  verified definition and should not be adopted.

The operator's own exported data independently contains the same armor reference
endpoints for models 4247 and 5293. For example, `54.7 + .12 * (58.7 - 54.7)` rounds
to `55.2`, consistent with the Python Casque screenshot. This is a formula cross-check
using a screenshot percentage, not a captured current item observation.

Important discrepancies remain:

- Armor reinforcement is divided by ten in the reference's catalog (local raw
  endpoints 138/144 are presented as 13.8/14.4). The scale must be explicit when
  validated, rather than silently changing raw source values.
- For accessory model 1888, the inspected catalog has physical base 14.8 and magic
  base 15.4, increment .29, and no upper reference for either. Our independently
  exported table has physical and magical ranges both 14.8–15.4. The reference can
  display roll percentages without applying them to these accessory values.
  These two interpretations must be reconciled before declaring the formula correct.
- Attack interpolation adds an extra five to the upper reference. Inspection alone
  does not establish a game-mechanics reason for this adjustment.
- Broad flattening of otherwise unclassified numeric fields explains how the
  reference can show unlabeled floating-point lines. PhMon must preserve those
  inputs as diagnostics without displaying unlabeled numbers.

The reference is useful evidence of its behavior; it is not sufficient authority to
copy every formula or claim game correctness. In particular, do not reverse the
meaning of raw client-table columns merely to match the screenshot.

## PhMon correction and next gate

Plugin 1.2.5 records item API evidence schema 2. Integer-keyed dictionaries become
ordered `mapping_entries` with `key_type`, decimal `key`, and bounded `value`. This
preserves IDs, zeros, integer precision and the distinction between an integer key
and the same textual key. String-only dictionaries retain their existing shape.
Additional attribute/option field aliases are collected as uninterpreted evidence.

`api_field_types` records source field names, types, collection sizes, and sampled
dictionary-key types without sending unknown values or credential fields. This also
distinguishes a genuinely empty source map from content previously lost during
serialization. Bounds remain 32 entries, nesting depth 3, strings 256 characters,
8 KiB of raw evidence and 2 KiB of structural metadata per item. Schema diagnostics
are limited to 64 source fields. The protocol's existing container/frame limits
still apply. No unverified evidence is automatically promoted into `instance`.

Regression tests cover integer/string ID collisions, large integers, order, zero,
empty/missing maps, field aliases, source immutability, diagnostic bounds, secret
exclusion and continued refusal to treat evidence as verified instance data.

Next: operator loads 1.2.5; inspect fresh `api_field_types` and `api_fields` for
the screenshot items, armor/weapon/shield/accessories and storage. Confirm actual
attribute IDs, option IDs/values, units and freshness. Prefer complete API-backed
observations where verified; use packet parsing for missing inputs. Then implement
the backend's typed API-instance conversion and verified family calculations with
independent expected values. No real character action is needed to obtain the next
API snapshot. Rolled tooltip completion remains open.
