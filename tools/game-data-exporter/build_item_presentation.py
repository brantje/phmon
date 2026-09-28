"""Build the backend's compact item catalog from an operator-owned bundle.

Usage: python build_item_presentation.py BUNDLE ASSET_INDEX OUTPUT [MEDIA_PK2]
Optional Media reads static facts when upgrading an older bundle; IDs and codes
must match the bundle. No source paths or archive content are included in output.
"""
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent / 'src'))
from phmon_game_exporter.item_metadata import item_metadata
from phmon_game_exporter.exporter import _archive_index, _item_shards, _lines
from phmon_game_exporter.pk2 import PK2Archive


def build(bundle, asset_index, output, media_path=None):
    catalog = json.loads((Path(bundle) / 'catalogs/items.json').read_text('utf-8'))
    assets = json.loads(Path(asset_index).read_text('utf-8'))
    dataset = catalog['datasetId']
    if dataset != assets['datasetId']:
        raise ValueError('asset and catalog datasets differ')
    icons = {key: '/' + entry['url'] for entry in assets['files'] for key in entry['assetKeys']}
    entity_catalog = json.loads((Path(bundle) / 'catalogs' / 'entities.json').read_text('utf-8'))
    if entity_catalog.get('datasetId') != dataset or entity_catalog.get('family') != 'entities':
        raise ValueError('entity catalog belongs to another dataset')
    extra = {}
    if media_path:
        with PK2Archive(media_path) as media:
            index = _archive_index(media.inventory().entries)
            for path, entry in _item_shards(media, index):
                lines, _ = _lines(media.read_payload(entry), path)
                for line in lines:
                    fields = line.split('\t')
                    extra[int(fields[1])] = (fields[2], item_metadata(fields))
    records = {}
    for row in catalog['records']:
        presentation = dict(row.get('presentation', {}))
        if extra:
            code, presentation = extra[row['referenceId']]
            if code != row['code']:
                raise ValueError('source item identity does not match bundle')
            presentation = dict(presentation)
        name = row['name'].get('en')
        if name:
            presentation['name'] = name
        icon = icons.get(row.get('assetKey'))
        if icon:
            presentation['icon_url'] = icon
        records[str(row['referenceId'])] = {'code': row['code'], 'presentation': presentation}
    magic_options = {}
    magic_path = Path(bundle) / 'catalogs' / 'magicOptions.json'
    if magic_path.is_file():
        catalog = json.loads(magic_path.read_text('utf-8'))
        if catalog.get('datasetId') != dataset or catalog.get('family') != 'magicOptions':
            raise ValueError('magic option catalog belongs to another dataset')
        for row in catalog.get('records', []):
            option_id = row.get('referenceId')
            code = row.get('code')
            if not isinstance(option_id, int) or option_id <= 0 or not isinstance(code, str):
                continue
            definition = {'code': code}
            if row.get('label_status') == 'exact_localization_join' and isinstance(row.get('label'), str):
                definition['label'] = row['label']
            ranges = row.get('raw_ranges')
            if isinstance(ranges, list) and len(ranges) <= 3:
                definition['raw_ranges'] = ranges
            if isinstance(row.get('level'), int) and 0 <= row['level'] <= 255:
                definition['level'] = row['level']
            magic_options[str(option_id)] = definition
    payload = {'dataset_id': dataset, 'items': records}
    character_portraits = {}
    for row in entity_catalog.get('records', []):
        if row.get('portraitMappingStatus') != 'verified-phmonitor-model-v050':
            continue
        model = row.get('referenceId')
        code = row.get('code')
        portrait_url = icons.get(row.get('portraitAssetKey'))
        if not isinstance(model, int) or model <= 0 or not isinstance(code, str) or not portrait_url:
            raise ValueError('verified character portrait join is incomplete')
        if model in character_portraits:
            raise ValueError('duplicate character model portrait mapping')
        character_portraits[str(model)] = {'code': code, 'portrait_url': portrait_url}
    if character_portraits:
        payload['character_portraits'] = character_portraits
    if magic_options:
        payload['magic_options'] = magic_options
    Path(output).parent.mkdir(parents=True, exist_ok=True)
    Path(output).write_text(json.dumps(payload, separators=(',', ':')), 'utf-8')
    print(f'Wrote {len(records)} item records, {len(character_portraits)} character portraits and {len(magic_options)} raw magic option definitions for {dataset}')


if __name__ == '__main__':
    build(*sys.argv[1:])
