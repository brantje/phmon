"""Build a profile-scoped name catalog from exported raw client tables."""

import argparse
import json
from pathlib import Path
import re
import sys

sys.path.insert(0, str(Path(__file__).parent / "src"))
from phmon_game_exporter.optional_teleports import optional_teleport_names


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("textdata", type=Path)
    parser.add_argument("dataset")
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r"gamedata-[a-z0-9]{1,64}", args.dataset):
        parser.error("invalid dataset ID")
    names = optional_teleport_names(
        (args.textdata / "refoptionalteleport.txt").read_text(encoding="utf-16"),
        (args.textdata / "textdata_object.txt").read_text(encoding="utf-16"),
    )
    payload = {}
    if args.output.exists():
        payload = json.loads(args.output.read_text(encoding="utf-8"))
    payload[args.dataset] = names
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(f"Wrote {len(names)} named Reverse return destinations for {args.dataset}")


if __name__ == "__main__":
    main()
