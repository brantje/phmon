"""Resolve enabled optional teleport names through exact English localization keys."""

from collections import defaultdict
import unicodedata


def optional_teleport_names(teleports: str, localization: str) -> list[str]:
    labels: dict[str, set[str]] = defaultdict(set)
    for line in localization.splitlines():
        if not line or line.startswith("//"):
            continue
        fields = line.split("\t")
        if len(fields) > 8 and fields[8].strip():
            labels[fields[1]].add(fields[8].strip())
    candidates: dict[str, list[str]] = defaultdict(list)
    ids = set()
    for line in teleports.splitlines():
        if not line or line.startswith("//"):
            continue
        fields = line.split("\t")
        if len(fields) != 19:
            raise ValueError("optional teleport row must have 19 fields")
        if fields[0] != "1":
            continue
        identity = int(fields[1])
        if identity <= 0 or identity in ids:
            raise ValueError("duplicate or invalid optional teleport ID")
        ids.add(identity)
        resolved = labels.get(fields[3], set())
        if len(resolved) != 1:
            continue
        name = next(iter(resolved))
        if (not name or name.casefold() in ("xxx", "null") or "?" in name
                or len(name.encode("utf-8")) > 100
                or any(unicodedata.category(char) == "Cc" for char in name)):
            continue
        candidates[name.casefold()].append(name)
    # A name-only API cannot distinguish multiple destinations with the same label.
    return sorted((names[0] for names in candidates.values() if len(names) == 1), key=str.casefold)
