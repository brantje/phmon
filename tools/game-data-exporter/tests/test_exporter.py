from __future__ import annotations

import hashlib
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
from phmon_game_exporter.pk2 import ArchiveInfo, Entry, PK2Archive
from phmon_game_exporter.preview import _map_sheet, _safe_bundle_file
from phmon_game_exporter.public_assets import ensure_public_asset_destination, validate_public_assets
from phmon_game_exporter.public_assets import materialize_public_assets
from .helpers import TEST_KEY, ddj_rgba, make_pk2


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
        "minimap_d/donwhang/dh_a01_floor01_127x126.ddj": ddj_rgba((60, 90, 140, 255)),
        "interface/loading/example.ddj": ddj_rgba((220, 190, 40, 255)),
        "interface/minimap/mm_sign_unique.ddj": ddj_rgba((210, 30, 200, 255)),
        "interface/character/char_ch_man1.ddj": ddj_rgba((180, 140, 110, 255)),
        "interface/targetwindow/tw_icon_normal.ddj": ddj_rgba((170, 120, 80, 0)),
        "interface/targetwindow/tw_icon_champion.ddj": ddj_rgba((230, 70, 60, 128)),
        "interface/targetwindow/tw_icon_giant.ddj": ddj_rgba((230, 220, 40, 255)),
        "icon/etc/europe_partymob.ddj": ddj_rgba((20, 130, 200, 255)),
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
    assert first["monsterTypeIconCount"] == 6
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
    cave_catalog = json.loads((bundle / "catalogs" / "caveMaps.json").read_text(encoding="utf-8"))
    assert cave_catalog["floorCount"] == 1
    assert cave_catalog["records"][0]["floorPrefix"] == "dh_a01_floor01"
    assert cave_catalog["records"][0]["x"] == 127
    symbols = json.loads((bundle / "catalogs" / "interfaceSymbols.json").read_text(encoding="utf-8"))
    assert symbols["records"][0]["symbolGroup"] == "minimapMarker"
    portraits = json.loads((bundle / "catalogs" / "portraits.json").read_text(encoding="utf-8"))
    assert portraits["records"][0]["mappingStatus"] == "unmapped-candidate"

    assert (public_assets / "icon" / "item" / "test_blade.png").is_file()
    assert (public_assets / "icon" / "skill" / "test.png").is_file()
    assert (public_assets / "minimap_d" / "donwhang" / "dh_a01_floor01_127x126.png").is_file()
    public_index = json.loads((public_assets / "asset-index.json").read_text(encoding="utf-8"))
    public_index_bytes = (public_assets / "asset-index.json").read_bytes()
    monster_catalog = json.loads((bundle / "catalogs" / "monsterTypes.json").read_text())
    assert monster_catalog["status"] == "parsed"
    assert [row["typeCode"] for row in monster_catalog["records"]] == [0, 1, 4, 16, 17, 20]
    monster_assets = {key: asset for asset in json.loads((bundle / "manifest.json").read_text())["assets"] for key in asset["semanticKeys"]}
    party_hashes = set()
    for row in monster_catalog["records"]:
        public_file = public_assets / row["publicAlias"]
        assert public_file.name.startswith(f'{row["typeCode"]}_')
        assert public_file.read_bytes() == (bundle / monster_assets[row["assetKey"]]["path"]).read_bytes()
        indexed = next(asset for asset in public_index["files"] if row["assetKey"] in asset["assetKeys"])
        assert indexed["path"] == row["publicAlias"]
        assert indexed["url"] == f'game-assets/{row["publicAlias"]}'
        with Image.open(public_file) as image:
            assert image.mode == "RGBA"
        if row["typeCode"] >= 16:
            assert row["role"] == "party_badge"
            assert row["rankTypeCode"] in (0, 1, 4)
            party_hashes.add(indexed["sha256"])
    assert len(party_hashes) == 1
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


def test_extracts_all_textdata_files_with_original_names_and_bytes(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    additions = {
        "Server_Dep/Silkroad/TextData/Nested/Unknown.BIN": b"\x00\xff\x80\r\n",
        "server_dep/silkroad/textdata/skilldata_9999.txt": b"unlisted, unparseable shard",
        "server_dep/silkroad/textdata/Empty.txt": b"",
        "server_dep/silkroad/textdata_backup/outside.txt": b"outside the directory",
        "server_dep/silkroad/other/outside.txt": b"outside the directory",
    }
    for path, payload in additions.items():
        _FakeArchive.payloads[path.casefold()] = payload
        _FakeArchive.media_entries += (Entry(2, path.rsplit("/", 1)[-1], path, 0, len(payload), 0),)
    directory = "server_dep/silkroad/textdata/UnusedDirectory"
    _FakeArchive.media_entries += (Entry(1, "UnusedDirectory", directory, 0, 0, 0),)
    public = tmp_path / "game-assets"
    output = tmp_path / "exports"
    result = exporter.export_dataset(source, output, "unused-test-key", public)
    textdata = Path(result["textdataPath"])
    expected = {
        entry.path[len(exporter.TEXT_ROOT):]: _FakeArchive.payloads[entry.path.casefold()]
        for entry in _FakeArchive.media_entries
        if entry.kind == 2 and entry.path.casefold().startswith(exporter.TEXT_ROOT)
    }
    actual = {path.relative_to(textdata).as_posix(): path.read_bytes() for path in textdata.rglob("*") if path.is_file()}
    assert actual == expected
    assert result["textdataFileCount"] == len(expected)
    assert result["textdataBytes"] == sum(map(len, expected.values()))
    report = json.loads((Path(result["auditPath"]) / "textdata.json").read_text())
    assert report["fileCount"] == result["textdataFileCount"]
    assert report["totalBytes"] == result["textdataBytes"]
    for record in report["files"]:
        assert record["sha256"] == hashlib.sha256(expected[record["path"]]).hexdigest()
        assert record["sizeBytes"] == len(expected[record["path"]])
    assert not (Path(result["bundlePath"]) / "textdata").exists()
    public_textdata = Path(result["publicTextdataPath"])
    assert public_textdata == public / "textdata"
    assert {path.relative_to(public_textdata).as_posix(): path.read_bytes() for path in public_textdata.rglob("*") if path.is_file()} == expected
    assert result["publicTextdataFileCount"] == len(expected)
    assert validate_public_assets(public)["textdataFilesValidated"] == len(expected)
    public_index = json.loads((public / "asset-index.json").read_text())
    raw_rows = [row for row in public_index["files"] if row.get("kind") == "textdata"]
    assert len(raw_rows) == len(expected)
    assert all(row["mediaType"] == "application/octet-stream" for row in raw_rows)
    assert all("sourceEntry" not in row for row in raw_rows)
    assert not (public / "audit").exists()
    assert not (public / "textdata.json").exists()
    assert exporter.export_dataset(source, output, "unused-test-key", public)["identicalBundleReused"]


@pytest.mark.parametrize("damage", ["modified", "unindexed", "audit-path"])
def test_public_textdata_rejects_corrupt_inputs_without_replacing_existing_tree(tmp_path, monkeypatch, damage):
    source = _make_sources(tmp_path, monkeypatch)
    public = tmp_path / "game-assets"
    result = exporter.export_dataset(source, tmp_path / "exports", "unused-test-key", public)
    index_bytes = (public / "asset-index.json").read_bytes()
    original_table = (public / "textdata" / "itemdata.txt").read_bytes()
    textdata = Path(result["textdataPath"])
    audit = Path(result["auditPath"])
    if damage == "modified":
        (textdata / "itemdata.txt").write_bytes(b"corrupted")
    elif damage == "unindexed":
        (textdata / "unindexed.txt").write_bytes(b"unexpected")
    else:
        report_path = audit / "textdata.json"
        report = json.loads(report_path.read_text())
        report["files"][0]["path"] = "../escape.txt"
        report_path.write_text(json.dumps(report))
    with pytest.raises(ValueError, match="checksum|extraction audit|safe relative"):
        materialize_public_assets(Path(result["bundlePath"]), audit, public, textdata=textdata)
    assert (public / "asset-index.json").read_bytes() == index_bytes
    assert (public / "textdata" / "itemdata.txt").read_bytes() == original_table
    assert not list(tmp_path.glob(".game-assets.*"))


def test_public_textdata_refresh_removes_obsolete_files(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    public = tmp_path / "game-assets"
    extra = "server_dep/silkroad/textdata/Old.txt"
    _FakeArchive.media_entries += (Entry(2, "Old.txt", extra, 0, 3, 0),)
    _FakeArchive.payloads[extra.casefold()] = b"old"
    exporter.export_dataset(source, tmp_path / "exports", "unused-test-key", public)
    assert (public / "textdata" / "Old.txt").read_bytes() == b"old"
    _FakeArchive.media_entries = tuple(entry for entry in _FakeArchive.media_entries if entry.path != extra)
    (source / "Media.pk2").write_bytes(b"fixture media archive without old table")
    refreshed = exporter.export_dataset(source, tmp_path / "exports", "unused-test-key", public)
    assert not (public / "textdata" / "Old.txt").exists()
    assert validate_public_assets(public)["textdataFilesValidated"] == refreshed["textdataFileCount"]


def test_public_validator_checks_raw_textdata_bytes(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    public = tmp_path / "game-assets"
    exporter.export_dataset(source, tmp_path / "exports", "unused-test-key", public)
    table = public / "textdata" / "itemdata.txt"
    payload = table.read_bytes()
    table.write_bytes(bytes([payload[0] ^ 1]) + payload[1:])
    with pytest.raises(ValueError, match="wrong checksum: textdata/itemdata.txt"):
        validate_public_assets(public)


def test_raw_only_publication_preserves_all_existing_artwork(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    public = tmp_path / "game-assets"
    result = exporter.export_dataset(source, tmp_path / "exports", "unused-test-key", public)
    old_index = json.loads((public / "asset-index.json").read_text())
    art = {row["path"]: (row, (public / row["path"]).read_bytes()) for row in old_index["files"] if row.get("kind") != "textdata"}
    raw = Path(result["textdataPath"])
    (raw / "itemdata.txt").write_bytes(b"new raw bytes")
    audit = Path(result["auditPath"])
    report_path = audit / "textdata.json"
    report = json.loads(report_path.read_text())
    table = next(row for row in report["files"] if row["path"] == "itemdata.txt")
    table.update(sizeBytes=13, sha256=hashlib.sha256(b"new raw bytes").hexdigest())
    report_path.write_text(json.dumps(report))
    materialize_public_assets(Path(result["bundlePath"]), audit, public, namespace="textdata", textdata=raw)
    new_index = json.loads((public / "asset-index.json").read_text())
    for row in new_index["files"]:
        if row["path"] in art:
            assert (row, (public / row["path"]).read_bytes()) == art[row["path"]]
    assert new_index["datasetId"] == old_index["datasetId"]
    assert (public / "textdata/itemdata.txt").read_bytes() == b"new raw bytes"
    validate_public_assets(public)


def test_extracts_original_payload_through_pk2_reader(tmp_path):
    archive_path = tmp_path / "Media.pk2"
    payload = b"\x00\xff\xfe\x80\r\n"
    make_pk2(archive_path, name="server_dep/silkroad/textdata/Nested/Raw.bin", payload=payload)
    destination = tmp_path / "textdata"
    with PK2Archive(archive_path, key=TEST_KEY) as archive:
        index = exporter._archive_index(archive.inventory().entries)
        report = exporter._extract_textdata(archive, index, destination)
    assert (destination / "Nested" / "Raw.bin").read_bytes() == payload
    assert report["fileCount"] == 1


@pytest.mark.parametrize("relative", ["../outside.txt", "nested/../../outside.txt", "/absolute.txt", "C:/outside.txt", "nested\\outside.txt", "control\x00.txt"])
def test_rejects_unsafe_raw_textdata_paths(tmp_path, relative):
    path = exporter.TEXT_ROOT + relative
    entry = Entry(2, relative.rsplit("/", 1)[-1], path, 0, 0, 0)
    with pytest.raises(exporter.ExportError, match="unsafe textdata output path"):
        exporter._extract_textdata(_FakeArchive(tmp_path / "Media.pk2", key="unused"), {path.casefold(): entry}, tmp_path / "textdata")


@pytest.mark.parametrize("damage", ["modified", "missing-file", "missing-directory", "symlink"])
def test_reuse_rejects_damaged_textdata_and_preserves_existing_output(tmp_path, monkeypatch, damage):
    source = _make_sources(tmp_path, monkeypatch)
    output = tmp_path / "exports"
    result = exporter.export_dataset(source, output, "unused-test-key")
    textdata = Path(result["textdataPath"])
    table = textdata / "itemdata.txt"
    if damage == "modified":
        table.write_bytes(b"damaged")
    elif damage == "missing-file":
        table.unlink()
    elif damage == "missing-directory":
        shutil.rmtree(textdata)
    else:
        is_symlink = Path.is_symlink
        monkeypatch.setattr(Path, "is_symlink", lambda path: path == table or is_symlink(path))
    with pytest.raises(exporter.ExportError, match="textdata"):
        exporter.export_dataset(source, output, "unused-test-key")
    assert Path(result["bundlePath"]).is_dir()
    assert not list(output.glob(".*.staging-*"))
    assert not list(output.glob(".*.lock"))
    if damage == "modified":
        assert table.read_bytes() == b"damaged"


def test_normal_export_reports_missing_monster_badge_without_fabricating_assets(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    _FakeArchive.media_entries = tuple(entry for entry in _FakeArchive.media_entries if entry.path != "icon/etc/europe_partymob.ddj")
    public = tmp_path / "game-assets"
    result = exporter.export_dataset(source, tmp_path / "out", "unused-test-key", public)
    bundle = Path(result["bundlePath"])
    catalog = json.loads((bundle / "catalogs" / "monsterTypes.json").read_text())
    assert result["monsterTypeIconCount"] == 3
    assert catalog["status"] == "partial"
    missing = [row for row in catalog["records"] if row["assetKey"] is None]
    assert [row["typeCode"] for row in missing] == [16, 17, 20]
    assert all(row["assetReferenceStatus"] == "missing-source-asset" and row["publicAlias"] is None for row in missing)
    unresolved = json.loads((Path(result["auditPath"]) / "unresolved.json").read_text())
    assert next(row for row in unresolved if row["family"] == "monsterTypes")["typeCodes"] == [16, 17, 20]
    assert len(list((public / "monster-types").glob("*.png"))) == 3
    validate_bundle(bundle)
    validate_public_assets(public)


def test_unsafe_monster_public_alias_cannot_escape_or_replace_existing_export(tmp_path, monkeypatch):
    source = _make_sources(tmp_path, monkeypatch)
    public = tmp_path / "game-assets"
    result = exporter.export_dataset(source, tmp_path / "out", "unused-test-key", public)
    before = (public / "asset-index.json").read_bytes()
    audit = Path(result["auditPath"])
    rows = json.loads((audit / "assets.json").read_text())
    next(row for row in rows if "publicAlias" in row)["publicAlias"] = "../escaped.png"
    (audit / "assets.json").write_text(json.dumps(rows))
    with pytest.raises(ValueError, match="safe relative"):
        materialize_public_assets(Path(result["bundlePath"]), audit, public)
    assert (public / "asset-index.json").read_bytes() == before
    assert not (tmp_path / "escaped.png").exists()


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
