"""Join monster resource names to model IDs and publish software-rendered artwork."""
from __future__ import annotations

import hashlib
import re
from collections import defaultdict
from pathlib import PurePosixPath

from .models import ModelError
from .monster_render import RENDERER_VERSION, RENDER_SIZE, render_resource, source_path
from .textures import TextureError
from .pk2 import PK2Error

MODEL_NAME = re.compile(r"[a-z0-9][a-z0-9_-]{0,127}\Z")


def monster_target(fields):
    """Use the exact AssocFileObj128 join, not a code-to-filename guess."""
    if not fields[2].startswith("MOB_"):
        return None
    value = fields[52].strip()
    if value.casefold() in ("", "xxx", "null", "0"):
        return None
    path = source_path(value, "res")
    if not path.endswith((".bsr", ".cpd")):
        return None
    name = PurePosixPath(path).stem
    if not MODEL_NAME.fullmatch(name):
        raise ModelError("Monster resource name is not safe for a public PNG filename")
    return {"modelName": name, "resource": path, "referenceId": int(fields[1]), "code": fields[2]}


def monster_targets(fields, *, uniques_only=False, invalid=None):
    """Expand explicit resource joins, retaining malformed entries in a private audit."""
    if uniques_only and (fields[0] != "1" or fields[15] not in ("3", "8")):
        return []
    targets = []
    for value in fields[52].split(","):
        row = list(fields)
        row[52] = value
        try:
            target = monster_target(row)
        except ModelError as exc:
            if invalid is None:
                raise
            invalid.append({"modelName": None, "resources": [value.strip()],
                            "referenceIds": [int(fields[1])], "codes": [fields[2]],
                            "status": "invalid", "reason": str(exc)})
            continue
        if target is not None:
            targets.append(target)
    return targets


def export_monsters(*, archive, index, targets, selected, dataset_id, bundle, assets, asset_audit,
                    collection_errors=()):
    """Render valid model groups; keep invalid joins separate from unsupported models."""
    paths = defaultdict(set)
    for target in targets:
        paths[target["modelName"]].add(target["resource"])
    groups = defaultdict(list)
    for target in targets:
        target = dict(target)
        if len(paths[target["modelName"]]) > 1:
            # Custom clients can reuse a basename in different model folders.
            # Retain the resource folder's name, never fall back to numeric IDs.
            folders = {path: PurePosixPath(path).parts[1:-1] for path in paths[target["modelName"]]}
            own = folders[target["resource"]]
            folder = None
            for depth in range(1, max(map(len, folders.values())) + 1):
                candidate = "_".join(own[-depth:])
                if sum("_".join(parts[-depth:]) == candidate for parts in folders.values()) == 1:
                    folder = candidate
                    break
            if folder is None:
                raise ModelError("Monster resources cannot be disambiguated by folder name")
            target["modelName"] = f'{folder}_{target["modelName"]}'
            if not MODEL_NAME.fullmatch(target["modelName"]):
                raise ModelError("Disambiguated monster name is not safe")
        groups[target["modelName"]].append(target)
    if selected is not None:
        unknown = set(selected) - groups.keys()
        if unknown:
            raise ModelError("Unknown monster model name(s): " + ", ".join(sorted(unknown)))
        groups = {name: groups[name] for name in selected}
    records, audit = [], list(collection_errors)
    for name, group in sorted(groups.items()):
        record = {"id": f"monster:{dataset_id}:{name}", "modelName": name,
                  "referenceIds": sorted({t["referenceId"] for t in group}),
                  "codes": sorted({t["code"] for t in group}), "assetKey": None,
                  "publicAlias": f"monsters/{name}.png", "status": "unsupported"}
        resources = {t["resource"] for t in group}
        evidence = {"modelName": name, "resources": sorted(resources)}
        try:
            if len(resources) != 1:
                raise ModelError("Model basename is ambiguous across different resource paths")
            if archive is None:
                raise ModelError("Data.pk2 is missing; model geometry cannot be read")
            resource = next(iter(resources))
            png, metadata = render_resource(archive, index, resource)
            digest = hashlib.sha256(png).hexdigest()
            relative = f"assets/images/{digest}.png"
            key = f"monster-art:{dataset_id}:{name}"
            if digest not in assets:
                path = bundle / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(png)
                assets[digest] = {"path": relative,"sha256": digest,"mediaType": "image/png",
                                  "sizeBytes": len(png),"width": RENDER_SIZE,"height": RENDER_SIZE,
                                  "semanticKeys": []}
            assets[digest]["semanticKeys"].append(key)
            asset_audit.append({"assetKey": key,"sourceEntry": resource,"status": "rendered",
                                "renderer": RENDERER_VERSION,"publicAlias": record["publicAlias"],
                                "sha256": digest})
            record.update(assetKey=key, status="rendered")
            evidence.update(status="rendered", **metadata)
        except (ModelError, TextureError, PK2Error) as exc:
            evidence.update(status="unsupported", reason=str(exc))
        records.append(record)
        audit.append(evidence)
    rendered = sum(r["status"] == "rendered" for r in records)
    return records, audit, {"models": len(records),"rendered": rendered,
                            "unsupported": len(records)-rendered,"invalid": len(collection_errors),
                            "renderer": RENDERER_VERSION,
                            "selection": list(selected) if selected is not None else "all-exact-resource-joins"}


def collect_monster_targets(rows, *, uniques_only=False, invalid=None):
    """Resolve inherited resource joins while preserving each source monster's identity."""
    by_code = {row[2]: row for row in rows}
    targets = []
    for row in rows:
        if uniques_only and (row[0] != "1" or row[15] not in ("3", "8")):
            continue
        resolved = list(row)
        base, visited = row, set()
        while base[52].strip().casefold() in ("", "xxx", "null", "0"):
            if base[2] in visited:
                if invalid is None:
                    raise ModelError("Cyclic monster base resource reference")
                invalid.append({"modelName": None, "resources": [],
                                "referenceIds": [int(row[1])], "codes": [row[2]],
                                "status": "invalid", "reason": "Cyclic monster base resource reference"})
                base = None
                break
            visited.add(base[2])
            base = by_code.get(base[4])
            if base is None:
                break
        if base is not None:
            resolved[52] = base[52]
            targets.extend(monster_targets(resolved, invalid=invalid))
    return targets
