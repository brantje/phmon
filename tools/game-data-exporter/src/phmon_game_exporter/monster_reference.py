"""Normalize client monster definitions, guide cells, and reference positions.

This module only reads exported text tables.  It never treats a reference point as
an observed spawn or assumes that a client guide cell is a precise spawn border.
"""

from __future__ import annotations

import math
import re
from collections import Counter, defaultdict
from typing import Any


CELL = re.compile(r"^(\d{1,3})x(\d{1,3})$")
CODE = re.compile(r"^MOB_[A-Za-z0-9_]+$")


def _lines(payload: bytes) -> list[tuple[int, str]]:
    for encoding in ("utf-16", "cp949", "utf-8-sig"):
        try:
            return list(enumerate(payload.decode(encoding).splitlines(), 1))
        except UnicodeDecodeError:
            continue
    raise ValueError("monster reference table has unsupported encoding")


def _cell(value: str) -> tuple[int, int] | None:
    match = CELL.fullmatch(value.strip())
    if not match:
        return None
    x, y = int(match[1]), int(match[2])
    return (x, y) if 0 <= x <= 255 and 0 <= y <= 255 else None


def _number(value: str) -> float | None:
    try:
        number = float(value)
    except ValueError:
        return None
    return number if math.isfinite(number) and -1_000_000 <= number <= 1_000_000 else None


def build_monster_reference(
    dataset_id: str,
    monster_rows: list[tuple[list[str], str, int]],
    object_text: dict[str, list[str]],
    tables: dict[str, bytes],
    catalog_version: str = "1.2.3",
) -> tuple[dict[str, Any], dict[str, Any]]:
    """Return a compact bundle catalog and a private, row-level audit."""
    problems: list[dict[str, Any]] = []
    counts: Counter[str] = Counter()
    definition_sources: list[dict[str, Any]] = []
    area_sources: list[dict[str, Any]] = []
    point_sources: list[dict[str, Any]] = []
    definitions: list[dict[str, Any]] = []
    by_model: dict[int, dict[str, Any]] = {}
    by_code: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for fields, path, line in monster_rows:
        counts["definitionRows"] += 1
        try:
            model = int(fields[1])
        except (IndexError, ValueError):
            problems.append({"source": path, "row": line, "reason": "invalid-model"})
            continue
        code = fields[2] if len(fields) > 2 else ""
        if not CODE.fullmatch(code):
            problems.append({"source": path, "row": line, "reason": "invalid-code", "model": model})
            continue
        enabled = fields[0] == "1"
        if not enabled:
            counts["disabledDefinitions"] += 1
        names = sorted({value for value in object_text.get(fields[5], [])
                        if value.strip() and value.strip() != "0"}) if len(fields) > 5 else []
        name = names[0] if len(names) == 1 else None
        if name is None:
            counts["ambiguousNames" if names else "missingNames"] += 1
        level = None
        if len(fields) > 57:
            try:
                candidate = int(fields[57])
                if 1 <= candidate <= 255:
                    level = candidate
            except ValueError:
                pass
        if level is None:
            counts["invalidLevels"] += 1
        row = {"id": f"monster-reference:{dataset_id}:{model}", "model_id": model,
               "code": code, "name": name, "level": level, "enabled": enabled}
        if model in by_model:
            problems.append({"source": path, "row": line, "reason": "duplicate-model", "model": model})
            continue
        definitions.append(row)
        definition_sources.append({"model": model, "code": code, "source": path, "row": line,
                                   "enabled": enabled, "nameJoin": "exact" if name else "ambiguous" if names else "missing",
                                   "levelJoin": "valid" if level else "invalid"})
        by_model[model] = row
        if enabled:
            by_code[code].append(row)
        if not enabled or name is None or level is None:
            problems.append({"source": path, "row": line, "reason": "definition-incomplete", "model": model,
                             "enabled": enabled, "nameStatus": "exact" if name else "ambiguous" if names else "missing",
                             "levelStatus": "valid" if level else "invalid"})
    for code, rows in by_code.items():
        if len(rows) > 1:
            counts["duplicateCodes"] += 1
            problems.append({"source": "characterdata", "reason": "duplicate-code", "code": code,
                             "models": [row["model_id"] for row in rows]})

    managers: dict[str, tuple[int, int]] = {}
    guide_rows: list[tuple[str, str, int]] = []
    guide_payload = tables.get("worldmapguidedata.txt")
    if guide_payload is None:
        problems.append({"source": "worldmapguidedata.txt", "reason": "missing-table"})
    else:
        section = ""
        for line, text in _lines(guide_payload):
            fields = text.split("\t")
            if fields[0] == "#section":
                section = fields[1].strip() if len(fields) > 1 else ""
                continue
            if not text or text.startswith("//"):
                continue
            if section == "MAP_MANAGER" and len(fields) >= 6 and fields[0] == "1":
                cell = _cell(fields[5])
                if cell is None or cell[0] < 1 or cell[1] < 1 or fields[1] in managers:
                    problems.append({"source": "worldmapguidedata.txt", "row": line, "reason": "invalid-manager"})
                else:
                    managers[fields[1]] = cell
            elif section == "MONSTER" and len(fields) >= 4 and fields[2].startswith("MOB_"):
                counts["guideRows"] += 1
                if fields[0] != "1":
                    counts["disabledGuideRows"] += 1
                    continue
                guide_rows.append((fields[2], fields[3], line))

    region_cells: dict[str, set[tuple[int, int]]] = defaultdict(set)
    region_sources: dict[str, list[int]] = defaultdict(list)
    region_payload = tables.get("worldmapguidedata_region.txt")
    if region_payload is None:
        problems.append({"source": "worldmapguidedata_region.txt", "reason": "missing-table"})
    else:
        for line, text in _lines(region_payload):
            if not text or text.startswith("//"):
                continue
            fields = text.split("\t")
            if len(fields) != 2 or not CODE.fullmatch(fields[0]):
                problems.append({"source": "worldmapguidedata_region.txt", "row": line, "reason": "invalid-row"})
                continue
            region_sources[fields[0]].append(line)
            for token in fields[1].split(","):
                if not token.strip():
                    continue
                cell = _cell(token)
                if cell is None:
                    problems.append({"source": "worldmapguidedata_region.txt", "row": line,
                                     "reason": "invalid-cell", "value": token[:80]})
                else:
                    region_cells[fields[0]].add(cell)

    areas: list[dict[str, Any]] = []
    grouped: dict[tuple[str, str], list[int]] = defaultdict(list)
    for code, group, line in guide_rows:
        grouped[(code, group)].append(line)
    for (code, group), lines in sorted(grouped.items()):
        matched = by_code.get(code, [])
        cell_size = managers.get(group)
        cells = sorted(region_cells.get(code, []))
        if len(matched) != 1 or cell_size is None or not cells:
            counts["unresolvedGuideGroups"] += 1
            problems.append({"source": "worldmapguidedata.txt", "rows": lines, "code": code, "group": group,
                             "reason": "ambiguous-definition" if len(matched) > 1 else "missing-definition" if not matched
                             else "missing-manager" if cell_size is None else "no-cells",
                             "regionRows": region_sources.get(code, [])})
            continue
        areas.append({"code": code, "model_id": matched[0]["model_id"], "group": group,
                      "precision": "guide-grid-cell",
                      "cells": [{"x": x, "y": y, "width": cell_size[0], "height": cell_size[1]}
                                for x, y in cells]})
        area_sources.append({"code": code, "group": group, "source": "worldmapguidedata.txt",
                             "rows": lines, "regionSource": "worldmapguidedata_region.txt",
                             "regionRows": region_sources.get(code, []), "cellSize": cell_size, "cells": cells,
                             "joinStatus": "exact"})
        counts["guideCells"] += len(cells)
        if len(lines) > 1 or len(region_sources.get(code, [])) > 1:
            counts["mergedGuideGroups"] += 1

    points: list[dict[str, Any]] = []
    seen_points: set[tuple[int, int, float, float, float]] = set()
    position_payload = tables.get("npcpos.txt")
    if position_payload is None:
        problems.append({"source": "npcpos.txt", "reason": "missing-table"})
    else:
        for line, text in _lines(position_payload):
            if not text or text.startswith("//"):
                continue
            counts["positionRows"] += 1
            fields = text.split("\t")
            try:
                model, region = int(fields[0]), int(fields[1])
            except (IndexError, ValueError):
                problems.append({"source": "npcpos.txt", "row": line, "reason": "invalid-identity"})
                continue
            definition = by_model.get(model)
            if definition is None or not definition["enabled"]:
                counts["unresolvedPositionJoins"] += 1
                problems.append({"source": "npcpos.txt", "row": line, "reason": "unresolved-model", "model": model})
                continue
            numbers = [_number(value) for value in fields[2:5]]
            if len(fields) != 5 or len(numbers) != 3 or any(value is None for value in numbers) or region == 0 or region < -32768 or region > 65535:
                counts["invalidPositionRows"] += 1
                problems.append({"source": "npcpos.txt", "row": line, "reason": "invalid-position", "model": model})
                continue
            raw_x, raw_z, raw_y = numbers
            key = (model, region, raw_x, raw_y, raw_z)
            if key in seen_points:
                counts["duplicatePositionRows"] += 1
                continue
            seen_points.add(key)
            points.append({"model_id": model, "code": definition["code"], "region": region,
                           "raw_x": raw_x, "raw_y": raw_y, "raw_z": raw_z,
                           "precision": "client-reference-point",
                           "coordinate_kind": "client-region-local" if region > 0 and region < 32767 else "client-interior"})
            point_sources.append({"model": model, "code": definition["code"], "source": "npcpos.txt",
                                  "row": line, "rawRegion": region, "rawX": raw_x, "rawHeight": raw_z,
                                  "rawY": raw_y, "joinStatus": "exact"})
    definitions.sort(key=lambda row: row["model_id"])
    points.sort(key=lambda row: (row["model_id"], row["region"], row["raw_x"], row["raw_y"], row["raw_z"]))
    counts["definitions"] = len(definitions)
    counts["areas"] = len(areas)
    counts["points"] = len(points)
    catalog = {"catalogVersion": catalog_version, "datasetId": dataset_id, "family": "monsterReference",
               "status": "partial" if problems else "parsed", "records": definitions,
               "areas": areas, "points": points, "coverage": dict(sorted(counts.items()))}
    audit = {"datasetId": dataset_id, "sourceTables": sorted(tables), "coverage": catalog["coverage"],
             "definitions": definition_sources, "areas": area_sources, "points": point_sources,
             "unresolved": problems}
    return catalog, audit
