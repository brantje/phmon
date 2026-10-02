"""Bounded parsers for the local Joymax mesh, material, skeleton and animation formats.

Independent implementation; source/version evidence is in format-notes.md.
"""
from __future__ import annotations

import math
import struct


class ModelError(ValueError):
    """Unsupported or malformed model data; never substitute invented geometry."""


class Reader:
    def __init__(self, data: bytes, signature: bytes):
        if data[:12] != signature:
            raise ModelError(f"Unsupported signature: {data[:12]!r}")
        self.data = data
        self.offset = 12

    def read(self, fmt):
        fmt = "<" + fmt
        length = struct.calcsize(fmt)
        if self.offset + length > len(self.data):
            raise ModelError("Truncated model")
        values = struct.unpack_from(fmt, self.data, self.offset)
        self.offset += length
        if any(isinstance(v, float) and not math.isfinite(v) for v in values):
            raise ModelError("Non-finite model value")
        return values[0] if len(values) == 1 else list(values)

    def count(self, maximum=100_000):
        count = self.read("I")
        if count > maximum:
            raise ModelError("Model count exceeds parser limit")
        return count

    def string(self):
        length = self.count(4096)
        if self.offset + length > len(self.data):
            raise ModelError("Truncated string")
        try:
            value = self.data[self.offset : self.offset + length].decode("euc_kr")
        except UnicodeDecodeError as exc:
            raise ModelError("Invalid model string encoding") from exc
        self.offset += length
        return value

    def seek(self, offset):
        if not 12 <= offset < len(self.data):
            raise ModelError("Invalid section offset")
        self.offset = offset

    def skip(self, count):
        if count < 0 or self.offset + count > len(self.data):
            raise ModelError("Truncated model data")
        self.offset += count


def resource(data, *, details=False):
    r = Reader(data, b"JMXVRES 0109")
    header = r.read("13I")
    if header[9] != 0:
        raise ModelError("Resource mod/attachment data is not yet supported")
    r.seek(header[0])
    materials = []
    for _ in range(r.count(5)):
        materials.append({"id": r.read("I"), "path": r.string()})
    r.seek(header[1])
    meshes = []
    for _ in range(r.count(128)):
        meshes.append(r.string())
        if header[8] & 1:
            r.read("I")
    r.seek(header[2])
    has_skeleton = r.read("I")
    if has_skeleton not in (0, 1):
        raise ModelError("Unsupported skeleton flag")
    skeleton_path = r.string() if has_skeleton else None
    attachment_bone = r.string() if has_skeleton else ""
    r.seek(header[3])
    animation_version, animation_user_type = r.read("2I")
    if animation_version != 0x1000 or animation_user_type != 0:
        raise ModelError("Unsupported animation type layout")
    animations = [r.string() for _ in range(r.count(256))]
    result = (materials, meshes, skeleton_path, animations)
    return (*result, {"attachmentBone": attachment_bone}) if details else result


def compound_resource(data):
    r = Reader(data, b"JMXVCPD 0101")
    header = r.read("7I")
    r.seek(header[1])
    paths, corrections = [], []
    for _ in range(r.count(32)):
        declared = r.count(512)
        # Custom CPDs can retain the old length after renaming an equipment
        # path. The explicit .bsr terminator provides a bounded exact join.
        end = data.find(b".bsr", r.offset, min(r.offset + 512, len(data)))
        if end < 0:
            raise ModelError("Compound resource has no bounded BSR path")
        actual = end + 4 - r.offset
        if abs(actual - declared) > 16:
            raise ModelError("Compound resource path length differs excessively")
        raw = data[r.offset:r.offset + actual]
        try:
            path = raw.decode("ascii")
        except UnicodeDecodeError as exc:
            raise ModelError("Unsupported compound path encoding") from exc
        if not path.replace("\\", "/").startswith("res/"):
            raise ModelError("Unsafe compound resource path")
        paths.append(path)
        if actual != declared:
            corrections.append({"path": path, "delta": actual - declared})
        r.offset += actual
    if not paths:
        raise ModelError("Empty compound resource")
    return paths, corrections


def material_set(data):
    r = Reader(data, b"JMXVBMT 0102")
    materials = []
    for _ in range(r.count(64)):
        name = r.string()
        diffuse = r.read("4f")
        r.read("12f")  # ambient, specular, emissive
        r.read("f")
        flag = r.read("I")
        texture = r.string()
        r.read("f")
        r.read("2B")
        absolute = r.read("B")
        if flag & 0x2000:
            r.string()
            r.read("I")
        materials.append({"name": name, "diffuse": diffuse, "flag": flag,
                          "texture": texture, "absolute": bool(absolute)})
    return materials


def mesh(data):
    r = Reader(data, b"JMXVBMS 0110")
    header = r.read("12I")
    if r.read("I") != 1:
        raise ModelError("Only one subprimitive is supported")
    flag = r.read("I")
    r.read("I")
    name, material = r.string(), r.string()
    r.read("I")
    if flag & ~0x1400:
        raise ModelError(f"Unsupported vertex flags: {flag:#x}")
    r.seek(header[0])
    count = r.count(65_535)
    positions, normals, uvs = [], [], []
    for _ in range(count):
        positions.extend(r.read("3f"))
        normals.extend(r.read("3f"))
        uvs.extend(r.read("2f"))
        if flag & 0x400:
            r.read("2f")
        r.read("f2I")
    r.seek(header[1])
    bones = [r.string() for _ in range(r.count(256))]
    joints, weights = [], []
    if bones:
        for _ in range(count):
            a, wa, b, wb = r.read("BHBH")
            total = wa + wb
            if (wa and a >= len(bones)) or (wb and b >= len(bones)) or not total:
                raise ModelError("Invalid skin influence")
            joints.extend([a if wa else 0, b if wb else 0, 0, 0])
            weights.extend([wa / total, wb / total, 0, 0])
    # Faces immediately follow the skin block. Some operator-edited meshes have
    # renamed bone strings without updating subsequent header offsets. Walking
    # the documented layout preserves the actual count and indices in that case.
    face_offset_delta = r.offset - header[2]
    if abs(face_offset_delta) > 4096:
        raise ModelError("Mesh face offset differs excessively from parsed skin block")
    indices = []
    for _ in range(r.count()):
        face = r.read("3H")
        if max(face) >= count:
            raise ModelError("Face references missing vertex")
        indices.extend(face)
    return {"name": name, "material": material, "positions": positions,
            "normals": normals, "uvs": uvs, "indices": indices,
            "bones": bones, "joints": joints, "weights": weights,
            "faceOffsetDelta": face_offset_delta}


def skeleton(data):
    r = Reader(data, b"JMXVBSK 0101")
    bones = []
    for _ in range(r.count(256)):
        r.read("B")
        name, parent = r.string(), r.string()
        quaternion, position = r.read("4f"), r.read("3f")
        r.skip(56)  # unused origin/local transforms can contain NaNs in custom files
        for _ in range(r.count(256)):
            r.string()
        bones.append({"name": name, "parent": parent,
                      "quaternion": quaternion, "position": position})
    return bones


def animation(data):
    r = Reader(data, b"JMXVBAN 0102")
    r.read("2I")
    name = r.string()
    duration, fps, loop = r.read("3I")
    timings = [r.read("I") for _ in range(r.count(10_000))]
    tracks = []
    for _ in range(r.count(256)):
        bone = r.string()
        count = r.count(10_000)
        if count != len(timings):
            raise ModelError("Unverified animation timing layout")
        quaternions, positions = [], []
        for _ in range(count):
            quaternions.extend(r.read("4f"))
            positions.extend(r.read("3f"))
        tracks.append({"bone": bone, "quaternions": quaternions, "positions": positions})
    return {"name": name, "duration": duration, "fps": fps,
            "loop": loop, "timings": timings, "tracks": tracks}
