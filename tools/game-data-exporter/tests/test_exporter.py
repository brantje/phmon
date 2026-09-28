from __future__ import annotations

import json
import shutil
from io import BytesIO
from pathlib import Path

import pytest
from PIL import Image

from phmon_game_exporter import exporter
from phmon_game_exporter.cli import validate_bundle
from phmon_game_exporter.item_metadata import item_metadata, magic_option_definitions
from phmon_game_exporter.portrait_mapping import portrait_for_model, portrait_source_path
from phmon_game_exporter.pk2 import ArchiveInfo, Entry
from phmon_game_exporter.preview import _map_sheet, _safe_bundle_file
from phmon_game_exporter.public_assets import ensure_public_asset_destination, validate_public_assets
from .helpers import ddj_rgba


@pytest.mark.parametrize(
    ("model", "filename"),
    [
        (1907, "char_ch_man1.png"),
        (1919, "char_ch_man13.png"),
        (1920, "char_ch_woman1.png"),
        (1932, "char_ch_woman13.png"),
        (14717, "char_eu_man1.png"),
        (14729, "char_eu_man13.png"),
        (14875, "char_eu_man1.png"),
        (14887, "char_eu_man13.png"),
        (14730, "char_eu_woman1.png"),
        (14742, "char_eu_woman13.png"),
        (14888, "char_eu_woman1.png"),
        (14900, "char_eu_woman13.png"),
    ],
)
def test_verified_phmonitor_character_portrait_model_ranges(model, filename):
    assert portrait_for_model(model)[3] == filename


@pytest.mark.parametrize("model", [0, -1, 1906, 1920 + 13, 14716, 14901, True, "1907"])
def test_unknown_or_invalid_models_have_no_portrait_mapping(model):
    assert portrait_for_model(model) is None


def test_verified_portrait_mapping_targets_source_ddj_before_png_conversion():
    assert portrait_source_path(1907) == "interface/character/char_ch_man1.ddj"
    assert portrait_source_path(14900) == "interface/character/char_eu_woman13.ddj"


def _text(value: str) -> bytes:
    return value.encode("utf-16")


def test_magic_option_export_keeps_exact_codes_and_unresolved_scaling() -> None:
    packed = (5 << 16) | 1
    rows = [
        "\t".join(["1", "9", "MATTR_INT", "x", "2", "x", "x", "x", str(packed), "0", "0"]),
        "\t".join(["1", "10", "MATTR_UNKNOWN", "x", "1", "x", "x", "x", "0", "0", "0"]),
    ]
    records, audit = magic_option_definitions(rows, {"MATTR_INT": ["Int Increase"]})
    assert records == [
        {
            "id": "magic-option:9",
            "referenceId": 9,
            "code": "MATTR_INT",
            "level": 2,
            "raw_ranges": [{"minimum": "1", "maximum": "5"}],
            "label_status": "exact_localization_join",
            "label": "Int Increase",
        },
        {
            "id": "magic-option:10",
            "referenceId": 10,
            "code": "MATTR_UNKNOWN",
            "level": 1,
            "raw_ranges": [],
            "label_status": "unresolved",
        },
    ]
    assert audit["valueScaleStatus"] == "unresolved"
    assert audit["labelsResolved"] == 1


def test_magic_option_export_drops_duplicate_ids_and_conflicting_labels() -> None:
    row = "\t".join(["1", "9", "MATTR_INT", "x", "2", "x", "x", "x", "0", "0", "0"])
    records, audit = magic_option_definitions(
        [row, row], {"MATTR_INT": ["First label", "Different label"]}
    )
    assert records == []
    assert audit["duplicateIds"] == [9]


def test_item_reference_ranges_export_without_calculating_instance_stats() -> None:
    fields = ["0"] * 126
    fields[9:13] = ["3", "1", "1", "1"]
    fields[14], fields[15] = "0", "0"
    fields[58:62] = ["0", "0", "0", "1"]
    fields[63:68] = ["14.200", "20.600", "50.0", "60.0", "1.25"]
    presentation = item_metadata(fields)
    assert presentation["reference_stats"]["durability"] == {"min": "14.2", "max": "20.6"}
    assert presentation["reference_stats"]["phy_def_pwr"] == {
        "min": "50", "max": "60", "increment": "1.25"
    }
    assert "phy_def_pwr" not in presentation


class _FakeArchive:
    payloads: dict[str, bytes] = {}
    media_entries: tuple[Entry, ...] = ()

    def __init__(self, path, *, key):
        self.path = Path(path)

    def __enter__(self):
        return self

    def __exit__(self, *args):
        return None

    def inventory(self):
        entries = self.media_entries if self.path.name.casefold() == "media.pk2" else ()
        counts = {".ddj": sum(1 for entry in entries if entry.name.casefold().endswith(".ddj"))}
        return ArchiveInfo(str(self.path), self.path.stat().st_size, True, True, len(entries), 0, len(entries), counts, entries)

    def read_payload(self, entry):
        return self.payloads[entry.path.casefold()]


def _make_item_row() -> str:
    fields = ["0"] * 161
    fields[1] = "44"
    fields[2] = "ITEM_TEST_BLADE"
    fields[5] = "SN_ITEM_TEST_BLADE"
    fields[6] = "SN_ITEM_TEST_BLADE_TT_DESC"
    fields[54] = r"item\test_blade.ddj"
    return "\t".join(fields)


def _make_skill_row() -> str:
    fields = ["0"] * 118
    fields[1] = "9001"
    fields[3] = "SKILL_TEST_01"
    fields[61] = r"skill\test.ddj"
    fields[62] = "SN_SKILL_TEST"
    fields[64] = "SN_SKILL_TEST_DESC"
    return "\t".join(fields)


def _make_sources(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    source = tmp_path / "GreatestSRO"
    source.mkdir()
    (source / "Media.pk2").write_bytes(b"synthetic media archive marker")
    (source / "Map.pk2").write_bytes(b"synthetic map archive marker")
    paths = {
        "server_dep/silkroad/textdata/itemdata.txt": _text("ItemData_5000.txt\r\n"),
        "server_dep/silkroad/textdata/itemdata_5000.txt": _text(_make_item_row() + "\r\n"),
        "server_dep/silkroad/textdata/skilldata.txt": _text("SkillData_5000.txt\r\n"),
        "server_dep/silkroad/textdata/skilldata_5000.txt": _text(_make_skill_row() + "\r\n"),
        "server_dep/silkroad/textdata/skillmasterydata.txt": _text('"\tWeapon Type 1\tWeapon Type 2\tWeapon Type 3\tMastery Icon\tMastery Focus Icon\r\n257\tignored\tUIIT_STT_MASTERY_TEST\t10\tUIIT_STT_MASTERY_DESC\tignored\t0\t0\t0\t0\t0\ticon\\skillmastery\\test.ddj\ticon\\skillmastery\\test_focus.ddj\r\n'),
        "server_dep/silkroad/textdata/skillgroup.txt": _text("1\tignored\t257\tignored\t0\tUIIT_STT_GROUP_TEST\tskillgroup\\test.ddj\r\n"),
        "server_dep/silkroad/textdata/teleportdata.txt": _text("1\t1\tGATE_TEST_1\t0\tSN_ZONE_TEST\t1001\t0\t0\t0\t0\t0\t0\t0\t\r\n1\t2\tGATE_TEST_2\t0\tSN_ZONE_TEST\t1001\t0\t0\t0\t0\t0\t0\t0\t\r\n"),
        "server_dep/silkroad/textdata/teleportlink.txt": _text("1\t1\t2\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\r\n"),
        "server_dep/silkroad/textdata/characterdata.txt": _text("CharacterData_5000.txt\r\nCharacterData_5001.txt\r\n"),
        "server_dep/silkroad/textdata/characterdata_5000.txt": _text(""),
        "server_dep/silkroad/textdata/characterdata_5001.txt": _text(""),
        "server_dep/silkroad/textdata/textdata_equip&skill.txt": _text("\tSN_ITEM_TEST_BLADE\t\t0\t0\t0\t0\t0\tTest Blade\t\r\n\tSN_SKILL_TEST\t\t0\t0\t0\t0\t0\tTest Skill\tTest Skill\r\n\tSN_SKILL_TEST_DESC\t\t0\t0\t0\t0\t0\tVerified skill description\tVerified skill description\r\n\tUIIT_STT_MASTERY_TEST\t\t0\t0\t0\t0\t0\tTest mastery\tTest mastery\r\n\tUIIT_STT_MASTERY_DESC\t\t0\t0\t0\t0\t0\tVerified mastery description\tVerified mastery description\r\n\tUIIT_STT_GROUP_TEST\t\t0\t0\t0\t0\t0\tTest mastery group\tTest mastery group\r\n"),
        "server_dep/silkroad/textdata/textdata_object.txt": _text("\tSN_ZONE_TEST\t\t0\t0\t0\t0\t0\tTest Region\t\r\n"),
        "server_dep/silkroad/textdata/refregion.txt": _text("\t".join(["1001", "1", "1", "China", "Jangan", "0", "1001", "0", "0", "0", "", *(["0"] * 10)]) + "\r\n"),
        "icon/item/test_blade.ddj": ddj_rgba((220, 30, 40, 255)),
        "icon/skill/test.ddj": ddj_rgba((40, 190, 50, 255)),
        "icon/skillmastery/test.ddj": ddj_rgba((80, 180, 70, 255)),
        "icon/skillmastery/test_focus.ddj": ddj_rgba((80, 180, 120, 255)),
        "icon/skillgroup/test.ddj": ddj_rgba((40, 190, 90, 255)),
        "minimap/1x1.ddj": ddj_rgba((40, 80, 220, 255)),
        "minimap/arabia/1x1.ddj": ddj_rgba((30, 70, 210, 255)),
        "interface/loading/example.ddj": ddj_rgba((220, 190, 40, 255)),
        "interface/minimap/mm_sign_unique.ddj": ddj_rgba((210, 30, 200, 255)),
        "interface/character/char_ch_man1.ddj": ddj_rgba((180, 140, 110, 255)),
    }
    entries = []
    payloads = {}
    for offset, (path, data) in enumerate(paths.items()):
        name = path.rsplit("/", 1)[-1]
        entries.append(Entry(2, name, path, 0, len(data), 0))
        payloads[path.casefold()] = data
    _FakeArchive.payloads = payloads
    _FakeArchive.media_entries = tuple(entries)
    monkeypatch.setattr(exporter, "PK2Archive", _FakeArchive)
    return source


def test_exports_bundle_reuses_identical_bytes_and_copied_bundle_stands_alone(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    output = tmp_path / "exports"
    public_assets = tmp_path / "web" / "public" / "game-assets"
    first = exporter.export_dataset(source, output, "unused-test-key", public_assets)
    assert first["completionStatus"] == "incomplete"
    assert first["itemCount"] == 1
    assert first["mapTileCount"] == 2
    assert first["backgroundCount"] == 1
    assert first["skillCount"] == 1
    assert first["masteryCount"] == 1
    assert first["skillGroupCount"] == 1
    assert first["teleportCount"] == 2
    assert first["teleportLinkCount"] == 1
    assert first["portraitCandidateCount"] == 1
    assert first["interfaceSymbolCount"] == 1
    audit_tables = Path(first["auditPath"]) / "tables"
    assert (audit_tables / "entities-000001.json").is_file()
    assert (audit_tables / "entities-000002.json").is_file()

    bundle = Path(first["bundlePath"])
    report = validate_bundle(bundle)
    assert report["sourceKnowledgeRequired"] is False
    item_catalog = json.loads((bundle / "catalogs" / "items.json").read_text(encoding="utf-8"))
    assert item_catalog["records"][0]["name"] == {"en": "Test Blade"}
    assert "typeCodes" not in item_catalog["records"][0]
    skill_catalog = json.loads((bundle / "catalogs" / "skills.json").read_text(encoding="utf-8"))
    assert skill_catalog["records"][0]["name"] == {"en": "Test Skill"}
    assert skill_catalog["records"][0]["description"] == {"en": "Verified skill description"}
    mastery_catalog = json.loads((bundle / "catalogs" / "masteries.json").read_text(encoding="utf-8"))
    assert mastery_catalog["records"][0]["name"] == {"en": "Test mastery"}
    assert mastery_catalog["records"][0]["focusIconAssetKey"]
    group_catalog = json.loads((bundle / "catalogs" / "skillGroups.json").read_text(encoding="utf-8"))
    assert group_catalog["records"][0]["masteryId"] == mastery_catalog["records"][0]["id"]
    teleport_catalog = json.loads((bundle / "catalogs" / "teleports.json").read_text(encoding="utf-8"))
    assert teleport_catalog["records"][0]["name"] == {"en": "Test Region"}
    assert teleport_catalog["records"][0]["regionId"] == "region:" + first["datasetId"] + ":1001"
    assert teleport_catalog["links"] == [{"fromTeleportId": 1, "toTeleportId": 2}]
    maps_catalog = json.loads((bundle / "catalogs" / "maps.json").read_text(encoding="utf-8"))
    root_map_tile = next(row for row in maps_catalog["records"] if row["tileSetId"] == "tile-set-001")
    region_catalog = json.loads((bundle / "catalogs" / "regions.json").read_text(encoding="utf-8"))
    region = region_catalog["records"][0]
    assert region["gridX"] == root_map_tile["x"] == 1
    assert region["gridZ"] == root_map_tile["y"] == 1
    assert region["mapAssetKeys"] == [root_map_tile["assetKey"]]
    assert region["mapTileIndexStatus"] == "exact-grid-match"
    assert root_map_tile["rasterContentStatus"] == "has-nonblack-pixels"
    assert region["mapTileContentStatus"] == "has-nonblack-pixels"
    assert region["worldTransformStatus"] == "unvalidated"
    assert region_catalog["rootMinimapGridJoin"]["matchedRegionRecords"] == 1
    assert "exact pairs only" in maps_catalog["coordinateSemantics"]
    assert maps_catalog["tileSetOrientations"][0]["status"] == "insufficient-adjacencies"
    assert maps_catalog["uniformOpaqueBlackTileCount"] == 0
    symbols = json.loads((bundle / "catalogs" / "interfaceSymbols.json").read_text(encoding="utf-8"))
    assert symbols["records"][0]["symbolGroup"] == "minimapMarker"
    portraits = json.loads((bundle / "catalogs" / "portraits.json").read_text(encoding="utf-8"))
    assert portraits["records"][0]["mappingStatus"] == "unmapped-candidate"

    assert (public_assets / "icon" / "item" / "test_blade.png").is_file()
    assert (public_assets / "icon" / "skill" / "test.png").is_file()
    public_index = json.loads((public_assets / "asset-index.json").read_text(encoding="utf-8"))
    public_index_bytes = (public_assets / "asset-index.json").read_bytes()
    assert public_index["format"] == "phmon-game-assets-index"
    assert all("sourceEntry" not in row and "archivePath" not in row for row in public_index["files"])
    assert any(row["url"] == "game-assets/icon/skill/test.png" for row in public_index["files"])
    assert validate_public_assets(public_assets)["sourceKnowledgeRequired"] is False

    second = exporter.export_dataset(source, output, "unused-test-key", public_assets)
    assert second["datasetId"] == first["datasetId"]
    assert second["identicalBundleReused"] is True
    assert (public_assets / "asset-index.json").read_bytes() == public_index_bytes

    copied = tmp_path / "copied-bundle"
    shutil.copytree(bundle, copied)
    copied_public_assets = tmp_path / "copied-game-assets"
    shutil.copytree(public_assets, copied_public_assets)
    shutil.rmtree(source)
    assert validate_bundle(copied)["assetsValidated"] == report["assetsValidated"]
    assert validate_public_assets(copied_public_assets)["publicFilesValidated"] == public_index["fileCount"]
    sheet = _map_sheet(copied, "tile-set-001")
    assert sheet.startswith(b"\x89PNG\r\n\x1a\n")
    with pytest.raises(ValueError, match="unsafe"):
        _safe_bundle_file(copied, "../outside.txt")


def test_uniform_black_map_raster_is_explicit_and_preview_hatched(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    black_path = "minimap/2x2.ddj"
    black_payload = ddj_rgba((0, 0, 0, 255))
    _FakeArchive.media_entries += (
        Entry(2, "2x2.ddj", black_path, 0, len(black_payload), 0),
    )
    _FakeArchive.payloads[black_path.casefold()] = black_payload

    # Keep the hashed staging filename under Windows MAX_PATH.
    result = exporter.export_dataset(source, tmp_path / "out", "unused-test-key")
    bundle = Path(result["bundlePath"])
    maps_catalog = json.loads((bundle / "catalogs" / "maps.json").read_text(encoding="utf-8"))
    black_tile = next(row for row in maps_catalog["records"] if (row["x"], row["y"]) == (2, 2))
    assert black_tile["rasterContentStatus"] == "uniform-opaque-black"
    assert maps_catalog["uniformOpaqueBlackTileCount"] == 1
    assert maps_catalog["status"] == "partial"

    regions = json.loads((bundle / "catalogs" / "regions.json").read_text(encoding="utf-8"))
    assert regions["records"][0]["mapTileContentStatus"] == "has-nonblack-pixels"
    audit_unresolved = json.loads((Path(result["auditPath"]) / "unresolved.json").read_text(encoding="utf-8"))
    map_gaps = [row for row in audit_unresolved if row.get("family") == "maps"]
    assert map_gaps[0]["recordCount"] == 1
    assert map_gaps[0]["tileRecords"] == [{"tileSetId": "tile-set-001", "x": 2, "y": 2}]

    validation = validate_bundle(bundle)
    assert validation["uniformOpaqueBlackMapTiles"] == 1
    sheet = Image.open(BytesIO(_map_sheet(bundle, "tile-set-001"))).convert("RGBA")
    assert sheet.getpixel((18, 18))[0] > 0


def test_map_sheet_places_increasing_y_above_smaller_indices(tmp_path):
    bundle = tmp_path / "bundle"
    maps_dir = bundle / "assets" / "maps"
    catalogs_dir = bundle / "catalogs"
    maps_dir.mkdir(parents=True)
    catalogs_dir.mkdir()
    tiles = [
        ("lower", 0, (220, 30, 40, 255)),
        ("upper", 1, (40, 80, 220, 255)),
    ]
    manifest_assets = []
    records = []
    for name, y, color in tiles:
        key = f"map:{name}"
        relative = f"assets/maps/{name}.png"
        Image.new("RGBA", (24, 24), color).save(bundle / relative)
        manifest_assets.append({"path": relative, "semanticKeys": [key]})
        records.append({"id": f"record:{name}", "tileSetId": "tile-set-001", "x": 0, "y": y, "assetKey": key})
    (bundle / "manifest.json").write_text(json.dumps({"assets": manifest_assets}), encoding="utf-8")
    (catalogs_dir / "maps.json").write_text(json.dumps({
        "records": records,
        "tileSetOrientations": [{"tileSetId": "tile-set-001", "status": "edge-continuity-supported", "yIncreasingDirection": "up"}],
    }), encoding="utf-8")

    sheet = Image.open(BytesIO(_map_sheet(bundle, "tile-set-001"))).convert("RGBA")

    assert sheet.size == (12, 24)
    assert sheet.getpixel((6, 6)) == (40, 80, 220, 255)
    assert sheet.getpixel((6, 18)) == (220, 30, 40, 255)


def test_map_sheet_reverses_x_when_increasing_direction_is_left(tmp_path):
    bundle = tmp_path / "bundle"
    maps_dir = bundle / "assets" / "maps"
    catalogs_dir = bundle / "catalogs"
    maps_dir.mkdir(parents=True)
    catalogs_dir.mkdir()
    tiles = [(0, "left", (220, 30, 40, 255)), (1, "right", (40, 80, 220, 255))]
    manifest_assets = []
    records = []
    for x, name, color in tiles:
        key = f"map:{name}"
        relative = f"assets/maps/{name}.png"
        Image.new("RGBA", (24, 24), color).save(bundle / relative)
        manifest_assets.append({"path": relative, "semanticKeys": [key]})
        records.append({"id": f"record:{name}", "tileSetId": "tile-set-001", "x": x, "y": 0, "assetKey": key})
    (bundle / "manifest.json").write_text(json.dumps({"assets": manifest_assets}), encoding="utf-8")
    (catalogs_dir / "maps.json").write_text(json.dumps({
        "records": records,
        "tileSetOrientations": [{"tileSetId": "tile-set-001", "status": "edge-continuity-supported", "xIncreasingDirection": "left"}],
    }), encoding="utf-8")

    sheet = Image.open(BytesIO(_map_sheet(bundle, "tile-set-001"))).convert("RGBA")

    assert sheet.getpixel((6, 6)) == (40, 80, 220, 255)
    assert sheet.getpixel((18, 6)) == (220, 30, 40, 255)


def test_output_path_rejects_symlinked_parent_before_resolution(tmp_path, monkeypatch):
    link = tmp_path / "linked-parent"
    is_symlink = Path.is_symlink

    def classify_link(path: Path) -> bool:
        return path == link or is_symlink(path)

    monkeypatch.setattr(Path, "is_symlink", classify_link)

    with pytest.raises(exporter.ExportError, match="symlink output path"):
        exporter._no_symlink_components(link / "new-output")
    with pytest.raises(ValueError, match="asset output path contains a symlink"):
        ensure_public_asset_destination(link / "new-public-assets")


def test_public_asset_export_preserves_unowned_destination(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    public_assets = tmp_path / "web" / "public" / "game-assets"
    public_assets.mkdir(parents=True)
    unrelated = public_assets / "keep.txt"
    unrelated.write_text("operator file", encoding="utf-8")

    with pytest.raises(ValueError, match="not owned"):
        exporter.export_dataset(source, tmp_path / "exports", "unused-test-key", public_assets)
    assert unrelated.read_text(encoding="utf-8") == "operator file"
    assert not (tmp_path / "exports").exists()
