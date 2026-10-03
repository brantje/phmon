from phmon_game_exporter.monster_reference import build_monster_reference


def row(model: int, code: str, level: str, token: str, enabled: str = "1"):
    fields = [""] * 58
    fields[0], fields[1], fields[2], fields[5], fields[57] = enabled, str(model), code, token, level
    return fields, "CharacterData_5000.txt", model


def text(*lines: str) -> bytes:
    return "\r\n".join(lines).encode("utf-16")


def test_reference_joins_variants_guide_cells_and_points_without_inventing_missing_rows():
    catalog, audit = build_monster_reference(
        "gamedata-fixture",
        [row(1954, "MOB_CH_TIGERWOMAN", "20", "tiger"),
         row(3796, "MOB_TK_EDIMMU_CLON", "61", "shakram"),
         row(3798, "MOB_TK_EDIMMU", "63", "edimmu"),
         row(4000, "MOB_DISABLED", "4", "off", "0")],
        {"tiger": ["Tiger Girl"], "shakram": ["Shakram"], "edimmu": ["Edimmu"]},
        {"worldmapguidedata.txt": text(
            "#section\t MAP_MANAGER", "1\tfield\t46x113\t17x26\t128x128\t4x4",
            "1\tdunhuang1\t127x128\t17x26\t256x256\t1x1",
            "#section\tMONSTER", "1\tShakram\tMOB_TK_EDIMMU_CLON\tfield",
            "1\tShakram\tMOB_TK_EDIMMU_CLON\tfield",
            "1\tEdimmu\tMOB_TK_EDIMMU\tdunhuang1",
            "0\tTiger\tMOB_CH_TIGERWOMAN\tfield"),
         "worldmapguidedata_region.txt": text(
             "MOB_TK_EDIMMU_CLON\t126x101,130x101",
             "MOB_TK_EDIMMU_CLON\t130x101,134x101",
             "MOB_TK_EDIMMU\t128x127"),
         "npcpos.txt": text("1954\t23712\t532.82\t1390.6\t938.17",
                            "3796\t25735\t216.1\t-4.57\t1864.31",
                            "3796\t25735\t216.1\t-4.57\t1864.31",
                            "9999\t25735\t10\t0\t20")},
    )
    definitions = {entry["code"]: entry for entry in catalog["records"]}
    assert [definitions[code]["level"] for code in
            ("MOB_CH_TIGERWOMAN", "MOB_TK_EDIMMU_CLON", "MOB_TK_EDIMMU")] == [20, 61, 63]
    assert definitions["MOB_DISABLED"]["enabled"] is False
    assert len(catalog["areas"]) == 2
    assert catalog["areas"][0]["cells"][0]["width"] == 1
    assert catalog["areas"][1]["cells"][0]["width"] == 4
    assert len(catalog["areas"][1]["cells"]) == 3
    assert len(catalog["points"]) == 2
    assert catalog["coverage"]["duplicatePositionRows"] == 1
    assert catalog["coverage"]["unresolvedPositionJoins"] == 1
    assert audit["points"][0]["rawHeight"] == 1390.6
    assert all("row" in entry for entry in audit["points"])


def test_reference_keeps_ambiguous_names_and_codes_unresolved():
    catalog, audit = build_monster_reference(
        "gamedata-another",
        [row(9001, "MOB_TEST", "12", "ambiguous"), row(9002, "MOB_TEST", "13", "other")],
        {"ambiguous": ["One", "Two"], "other": ["Other"]},
        {"worldmapguidedata.txt": text("#section\t MAP_MANAGER", "1\tfield\t0\t0\t128x128\t4x4",
                                       "#section\tMONSTER", "1\tTest\tMOB_TEST\tfield"),
         "worldmapguidedata_region.txt": text("MOB_TEST\t100x100"),
         "npcpos.txt": text("9001\t25735\t1\t0\t1")},
    )
    assert len(catalog["areas"]) == 0
    assert catalog["records"][0]["name"] is None
    assert catalog["coverage"]["duplicateCodes"] == 1
    assert any(entry["reason"] == "ambiguous-definition" for entry in audit["unresolved"])


def test_numeric_zero_localization_is_missing_not_a_display_name():
    catalog, audit = build_monster_reference(
        "gamedata-another", [row(9001, "MOB_TEST", "12", "zero")],
        {"zero": ["0"]}, {},
    )
    assert catalog["records"][0]["name"] is None
    assert catalog["coverage"]["missingNames"] == 1
    assert audit["definitions"][0]["nameJoin"] == "missing"
