"""Static vSRO item-table facts. Never substitute these for instance rolls.

Column contracts: RSBot RefObjCommon/RefObjItem (see format-notes.md).
Only the legacy equipment taxonomy and seal classes verified there are mapped.
"""

from decimal import Decimal, InvalidOperation
import re

def item_metadata(fields: list[str]) -> dict:
    if len(fields) < 62:
        return {}
    try:
        type1, type2, family, part = (int(fields[i]) for i in (9, 10, 11, 12))
        country, rarity, gender, strength, intelligence, item_class = (
            int(fields[i]) for i in (14, 15, 58, 59, 60, 61)
        )
    except ValueError:
        return {}
    if type1 != 3:
        return {}
    result = {'type_ids': [type1, type2, family, part], 'rarity': rarity}
    for index in (32, 34, 36, 38):
        try:
            if int(fields[index]) == 1 and int(fields[index + 1]) > 0:
                result['required_level'] = int(fields[index + 1])
        except ValueError:
            pass
    if type2 == 1:
        result['rare'] = rarity in (2, 3, 6, 7)
        if 1 <= item_class <= 30:
            result['degree'] = (item_class - 1) // 3 + 1
            if rarity == 2:
                result['seal'] = ('Seal of Star', 'Seal of Moon', 'Seal of Sun')[(item_class - 1) % 3]
        for key, value in (('required_strength', strength), ('required_intelligence', intelligence)):
            if value > 0:
                result[key] = value
        if country in (0, 1):
            result['required_race'] = ('Chinese', 'European')[country]
        armor = {1: 'Garment', 2: 'Protector', 3: 'Armor', 9: 'Robe', 10: 'Light armor', 11: 'Heavy armor'}
        if family in armor:
            result['sort_type'] = armor[family]
            mounting = {1: 'Head', 2: 'Chest', 3: 'Shoulders', 4: 'Hands', 5: 'Legs', 6: 'Feet'}
            if part in mounting:
                result['mounted_part'] = mounting[part]
            if gender in (0, 1):
                result['required_gender'] = ('Female', 'Male')[gender]
        elif family in (5, 12):
            if part in (1, 2, 3):
                result['sort_type'] = {1: 'Earring', 2: 'Necklace', 3: 'Ring'}[part]
        elif family == 4:
            result['sort_type'] = 'Shield'
        elif family == 6:
            result['sort_type'] = 'Weapon'
        reference_stats = _reference_stats(fields, family)
        if reference_stats:
            result['reference_stats'] = reference_stats
    elif type2 == 3:
        if family == 1:
            result['sort_type'] = 'Potion'
        elif family == 2:
            result['sort_type'] = 'Pill'
    return result


def _decimal_cell(fields: list[str], index: int) -> str | None:
    if index >= len(fields) or not fields[index].strip():
        return None
    try:
        value = Decimal(fields[index].strip())
    except InvalidOperation:
        return None
    if not value.is_finite() or value < 0 or value > Decimal('1000000000'):
        return None
    normalized = format(value.normalize(), 'f')
    return normalized.rstrip('0').rstrip('.') if '.' in normalized else normalized


def _range(fields: list[str], minimum: int, maximum: int, increment: int | None = None) -> dict | None:
    low, high = _decimal_cell(fields, minimum), _decimal_cell(fields, maximum)
    if low is None or high is None:
        return None
    result = {'min': low, 'max': high}
    if increment is not None:
        value = _decimal_cell(fields, increment)
        if value is not None:
            result['increment'] = value
    return result


def _reference_stats(fields: list[str], family: int) -> dict:
    """Export table ranges verbatim as reference data; never calculate rolls here."""
    result = {}
    if family in (1, 2, 3, 9, 10, 11):
        mappings = {
            'durability': (63, 64, None),
            'phy_def_pwr': (65, 66, 67),
            'parry_ratio': (68, 69, 70),
            'mag_def_pwr': (76, 77, 78),
            'phy_reinforce': (82, 83, None),
            'mag_reinforce': (84, 85, None),
        }
        for key, columns in mappings.items():
            value = _range(fields, *columns)
            if value is not None:
                result[key] = value
    elif family == 4:
        mappings = {
            'durability': (63, 64, None),
            'phy_def_pwr': (65, 66, 67),
            'block_ratio': (74, 75, None),
            'mag_def_pwr': (76, 77, 78),
            'phy_reinforce': (82, 83, None),
            'mag_reinforce': (84, 85, None),
        }
        for key, columns in mappings.items():
            value = _range(fields, *columns)
            if value is not None:
                result[key] = value
    elif family in (5, 12):
        for key, columns in {'phy_absorption': (71, 72), 'mag_absorption': (79, 80)}.items():
            value = _range(fields, *columns)
            if value is not None:
                result[key] = value
    elif family == 6:
        for key, columns in {
            'phy_atk_pwr_min': (95, 96),
            'phy_atk_pwr_max': (97, 98),
            'mag_atk_pwr_min': (100, 101),
            'mag_atk_pwr_max': (102, 103),
            'phy_reinforce_min': (105, 106),
            'phy_reinforce_max': (107, 108),
            'mag_reinforce_min': (109, 110),
            'mag_reinforce_max': (111, 112),
            'hit_ratio': (113, 114, 115),
            'critical_ratio': (116, 117),
            'durability': (63, 64),
        }.items():
            value = _range(fields, *columns)
            if value is not None:
                result[key] = value
    return result


def magic_option_definitions(lines: list[str], localized_labels: dict[str, list[str]]) -> tuple[list[dict], dict]:
    """Export raw option references; never invent a display label or value scale.

    The row indexes follow the pinned RefMagicOpt loader. The range cells are
    packed unsigned pairs (low 16 bits first, high 16 bits second). Display
    labels are attached only through an exact, unique localization-key match.
    """
    records = []
    malformed = 0
    duplicate_ids = set()
    seen = set()
    for line in lines:
        fields = line.split('\t')
        if len(fields) < 11:
            malformed += 1
            continue
        if fields[0].strip() not in ('1', 'true', 'True', 'TRUE'):
            continue
        try:
            option_id = int(fields[1], 10)
            level = int(fields[4], 10)
            if option_id <= 0 or option_id > 0xffffffff or level < 0 or level > 255:
                raise ValueError('range')
        except ValueError:
            malformed += 1
            continue
        group = fields[2].strip()
        if not re.fullmatch(r'[A-Za-z0-9_]{1,128}', group):
            malformed += 1
            continue
        if option_id in seen:
            duplicate_ids.add(option_id)
            continue
        seen.add(option_id)
        ranges = []
        valid = True
        for column in (8, 9, 10):
            try:
                packed = int(fields[column], 10)
                if packed < 0 or packed > 0xffffffff:
                    raise ValueError('range')
            except ValueError:
                valid = False
                break
            minimum, maximum = packed & 0xffff, packed >> 16
            if packed and minimum <= maximum:
                ranges.append({'minimum': str(minimum), 'maximum': str(maximum)})
        if not valid:
            malformed += 1
            continue
        record = {
            # Catalog records require stable string identities. Keep the client
            # numeric ID separately so consumers can match without coercion.
            'id': f'magic-option:{option_id}',
            'referenceId': option_id,
            'code': group,
            'level': level,
            'raw_ranges': ranges,
            'label_status': 'unresolved',
        }
        labels = localized_labels.get(group, [])
        unique_labels = sorted(set(value.strip() for value in labels if value.strip()))
        if len(unique_labels) == 1:
            record['label'] = unique_labels[0]
            record['label_status'] = 'exact_localization_join'
        elif len(unique_labels) > 1:
            record['label_status'] = 'conflicting_localization'
        records.append(record)
    audit = {
        'status': 'parsed' if not malformed and not duplicate_ids else 'partial',
        'rowCount': len(lines),
        'recordCount': len(records),
        'malformedRows': malformed,
        'duplicateIds': sorted(duplicate_ids),
        'labelsResolved': sum(1 for row in records if row['label_status'] == 'exact_localization_join'),
        'valueScaleStatus': 'unresolved',
    }
    records = [row for row in records if row['referenceId'] not in duplicate_ids]
    audit['recordCount'] = len(records)
    return records, audit
