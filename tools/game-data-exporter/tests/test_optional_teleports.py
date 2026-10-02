import json
from pathlib import Path
import subprocess
import sys

import pytest

from phmon_game_exporter.optional_teleports import optional_teleport_names


def row(identity, token, service=1):
    return "\t".join([str(service), str(identity), "????", token, "25000", "0", "0", "0", "1", "-1", "1", "0", "0", "-1", "xxx", "-1", "xxx", "-1", "xxx"])


def label(token, value):
    return "\t".join(["1", token, "", "", "", "", "", "", value])


def test_enabled_locations_resolve_exact_english_labels_only():
    rows = "\n".join([row(1, "SN_JANGAN"), row(2, "SN_DISABLED", 0), row(3, "xxx"), row(4, "SN_AMBIGUOUS"), row(5, "SN_CONTROL")])
    labels = "\n".join([label("SN_JANGAN", " Jangan "), label("SN_DISABLED", "Disabled"), label("SN_AMBIGUOUS", "A"), label("SN_AMBIGUOUS", "B"), label("SN_CONTROL", "bad\x7f")])
    assert optional_teleport_names(rows, labels) == ["Jangan"]


def test_duplicate_destination_names_are_ambiguous_for_name_only_api():
    assert optional_teleport_names("\n".join([row(1, "A"), row(2, "B")]), "\n".join([label("A", "Hotan"), label("B", "hotan")])) == []


def test_malformed_rows_and_duplicate_ids_are_rejected():
    with pytest.raises(ValueError, match="19 fields"):
        optional_teleport_names("1\t2", "")
    with pytest.raises(ValueError, match="duplicate"):
        optional_teleport_names("\n".join([row(1, "A"), row(1, "B")]), "")


def test_catalog_builder_merges_existing_utf8_names(tmp_path):
    textdata = tmp_path / "textdata"
    textdata.mkdir()
    (textdata / "refoptionalteleport.txt").write_text(
        row(1, "SN_JANGAN"), encoding="utf-16"
    )
    (textdata / "textdata_object.txt").write_text(
        label("SN_JANGAN", "Jångan"), encoding="utf-16"
    )
    output = tmp_path / "locations.json"
    output.write_text(
        json.dumps({"gamedata-other": ["旧名称"]}, ensure_ascii=False),
        encoding="utf-8",
    )
    script = Path(__file__).resolve().parents[1] / "build_reverse_return_locations.py"

    subprocess.run(
        [sys.executable, str(script), str(textdata), "gamedata-test", str(output)],
        check=True,
        capture_output=True,
        text=True,
    )

    assert json.loads(output.read_text(encoding="utf-8")) == {
        "gamedata-other": ["旧名称"],
        "gamedata-test": ["Jångan"],
    }
