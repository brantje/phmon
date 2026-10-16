"""Refresh the packaged item catalog in place from raw client textdata.

Usage: python refresh_item_catalog.py CATALOG_JSON TEXTDATA_DIR

Adds static tooltip facts (attack distance, max magic options, mastery
requirements, specific item group) to existing item presentations and value
ranges to magic options. Existing keys, names and icons are kept. A matched ID
whose item code differs from the catalog aborts the run before writing.
"""
import json
import os
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent / 'src'))
from phmon_game_exporter.item_metadata import item_metadata

NEW_KEYS = ('attack_distance', 'max_magic_options', 'mastery_requirements')


def read_table(path: Path) -> list[list[str]]:
    text = path.read_bytes().decode('utf-16-le').lstrip('﻿')
    return [line.split('\t') for line in text.splitlines() if line and not line.startswith(('#', '//'))]


def find(textdata: Path, name: str) -> Path:
    for entry in textdata.iterdir():
        if entry.name.lower() == name.lower():
            return entry
    raise FileNotFoundError(name)


def item_groups(textdata: Path) -> dict[tuple[int, int, int, int], str]:
    labels = {row[1]: row[8].strip() for row in read_table(find(textdata, 'textdata_object.txt')) if len(row) > 8}
    groups = {}
    for row in read_table(find(textdata, 'item_grouping.txt')):
        if len(row) < 6 or not row[1].isdigit():
            continue
        label = labels.get(row[5].strip())
        if label:
            groups[tuple(int(value) for value in row[1:5])] = label
    return groups


def option_ranges(textdata: Path) -> dict[str, tuple[int, int]]:
    ranges = {}
    for fields in read_table(find(textdata, 'magicoption.txt')):
        if len(fields) < 11 or fields[0].strip() not in ('1', 'true', 'True', 'TRUE'):
            continue
        try:
            option_id = int(fields[1])
            halves = []
            for column in (8, 9, 10):
                packed = int(fields[column])
                halves += [packed & 0xffff, packed >> 16]
        except ValueError:
            continue
        values = [value for value in halves if value]
        if values and min(values) < max(values):
            ranges[str(option_id)] = (min(values), max(values))
    return ranges


def source_items(textdata: Path) -> dict[int, tuple[str, dict]]:
    items = {}
    for shard in sorted(textdata.iterdir()):
        name = shard.name.lower()
        if not (name.startswith('itemdata_') and name.endswith('.txt')):
            continue
        for fields in read_table(shard):
            if len(fields) > 2 and fields[1].isdigit():
                items[int(fields[1])] = (fields[2], item_metadata(fields))
    return items


def refresh(catalog_path: Path, textdata: Path) -> dict:
    catalog = json.loads(catalog_path.read_text('utf-8'))
    source = source_items(textdata)
    groups = item_groups(textdata)
    for item_id, record in catalog['items'].items():
        found = source.get(int(item_id))
        if found and found[0] != record['code']:
            raise ValueError(f'item {item_id} code differs from the catalog')
    counts = {key: 0 for key in (*NEW_KEYS, 'item_group')}
    for item_id, record in catalog['items'].items():
        found = source.get(int(item_id))
        if not found:
            continue
        presentation = record['presentation']
        for key in NEW_KEYS:
            if key in found[1]:
                presentation[key] = found[1][key]
                counts[key] += 1
            else:
                presentation.pop(key, None)
        type_ids = found[1].get('type_ids')
        group = groups.get(tuple(type_ids)) if type_ids else None
        if group:
            presentation['item_group'] = group
            counts['item_group'] += 1
    ranges = option_ranges(textdata)
    counts['magic_option_ranges'] = 0
    for option_id, option in catalog['magic_options'].items():
        if option_id in ranges:
            option['value_min'], option['value_max'] = ranges[option_id]
            counts['magic_option_ranges'] += 1
    descriptor, temporary = tempfile.mkstemp(dir=catalog_path.parent, suffix='.tmp')
    try:
        with os.fdopen(descriptor, 'w', encoding='utf-8') as handle:
            json.dump(catalog, handle, ensure_ascii=False, separators=(',', ':'))
        # mkstemp creates the file 0600; the server image reads it as another user.
        os.chmod(temporary, catalog_path.stat().st_mode & 0o777)
        os.replace(temporary, catalog_path)
    except BaseException:
        Path(temporary).unlink(missing_ok=True)
        raise
    return counts


if __name__ == '__main__':
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    print(json.dumps(refresh(Path(sys.argv[1]), Path(sys.argv[2])), indent=2))
