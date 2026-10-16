"""Build a dataset-scoped pet reference (model -> level, max HP; level -> max EXP).

Columns come from the raw textdata tables copied by the exporter:
characterdata_*.txt (servername=2, model=1, level=57, max HP=59) and
leveldata.txt (level=0, EXP to next level=1). Verified against the Greatest
client: model 6142 (COS_P_WOLF_037) is level 37 with 6022 max HP, and level 37
needs 3,239,210 EXP, matching the in-game pet window.
"""

import argparse
import json
from pathlib import Path

PREFIXES = ("COS_P_", "COS_C_", "COS_T_", "COS_TRANSPORT_", "COS_FELLOW_")


def read_rows(path):
    return path.read_bytes().decode("utf-16", errors="replace").splitlines()


def build(textdata, dataset_id):
    pets = {}
    for path in sorted(textdata.glob("characterdata_*.txt")):
        for line in read_rows(path):
            columns = line.split("\t")
            if len(columns) < 60 or columns[0].strip() != "1":
                continue
            if not columns[2].startswith(PREFIXES):
                continue
            try:
                model, level, max_hp = int(columns[1]), int(columns[57]), int(columns[59])
            except ValueError:
                continue
            if model < 1 or level < 0 or max_hp < 1:
                continue
            pets[str(model)] = {"code": columns[2], "level": level, "max_hp": max_hp}
    exp = {}
    for line in read_rows(textdata / "leveldata.txt"):
        columns = line.split("\t")
        try:
            level, needed = int(columns[0]), int(columns[1])
        except (ValueError, IndexError):
            continue
        if level >= 1 and needed > 0:
            exp[str(level)] = needed
    return {
        "dataset_id": dataset_id,
        "pets": dict(sorted(pets.items(), key=lambda item: int(item[0]))),
        "exp_to_next_level": dict(sorted(exp.items(), key=lambda item: int(item[0]))),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--textdata", type=Path, required=True)
    parser.add_argument("--dataset-id", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    document = build(args.textdata, args.dataset_id)
    args.output.write_text(json.dumps(document, separators=(",", ":")) + "\n")
    print(len(document["pets"]), "pets,", len(document["exp_to_next_level"]), "levels")


if __name__ == "__main__":
    main()
