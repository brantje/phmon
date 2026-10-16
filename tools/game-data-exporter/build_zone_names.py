"""Build a profile-scoped region ID -> English zone name catalog from textzonename.txt."""

import argparse
import json
from pathlib import Path
import re


def zone_names(text):
    names = {}
    ambiguous = set()
    for line in text.splitlines():
        columns = line.split("\t")
        if len(columns) < 10 or columns[0].strip() != "1":
            continue
        try:
            region = int(columns[1])
        except ValueError:
            continue
        if region > 32767:
            region -= 65536
        if not -32768 <= region <= 32767:
            continue
        name = next(
            (value.strip() for value in (columns[9], columns[8]) if value.strip() not in ("", "0")),
            "",
        )
        if not name or len(name) > 100:
            continue
        key = str(region)
        if key in names and names[key] != name:
            ambiguous.add(key)
        names.setdefault(key, name)
    for key in ambiguous:
        names.pop(key, None)
    return dict(sorted(names.items(), key=lambda item: int(item[0])))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("textdata", type=Path)
    parser.add_argument("dataset")
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r"gamedata-[a-z0-9]{1,64}", args.dataset):
        parser.error("invalid dataset ID")
    names = zone_names((args.textdata / "textzonename.txt").read_text(encoding="utf-16"))
    payload = {}
    if args.output.exists():
        payload = json.loads(args.output.read_text(encoding="utf-8"))
    payload[args.dataset] = names
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(payload, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8"
    )
    print(f"Wrote {len(names)} region zone names for {args.dataset}")


if __name__ == "__main__":
    main()
