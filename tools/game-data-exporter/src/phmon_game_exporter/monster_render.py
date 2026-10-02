"""Offline CPU rasterization of local monster resources to transparent PNG.

No browser, GPU, Blender or Three.js is needed. Only the versions with verified
parsers are accepted. Unsupported resources remain explicit in the export audit.
"""
from __future__ import annotations

import hashlib
import io
import math
import re
from pathlib import PurePosixPath

import numpy as np
from PIL import Image

from . import models
from .models import ModelError
from .textures import ddj_to_png

RENDERER_VERSION = "phmon-software-v1"
RENDER_SIZE = 512
MAX_MODEL_VERTICES = 100_000
MAX_MODEL_TRIANGLES = 100_000


def source_path(value: str, prefix: str = "") -> str:
    value = value.replace("\\", "/").casefold()
    parts = value.split("/")
    if not value or len(value) > 512 or ":" in value or any(p in ("", ".", "..") for p in parts):
        raise ModelError("Unsafe model resource path")
    return value if not prefix or value.startswith(prefix + "/") else prefix + "/" + value


def _matrix(quaternion, position):
    q = np.asarray(quaternion, dtype=np.float64)
    length = np.linalg.norm(q)
    if not np.isfinite(length) or length < 1e-12:
        raise ModelError("Invalid bone quaternion")
    x, y, z, w = q / length
    matrix = np.eye(4)
    matrix[:3, :3] = [[1-2*(y*y+z*z), 2*(x*y-z*w), 2*(x*z+y*w)],
                     [2*(x*y+z*w), 1-2*(x*x+z*z), 2*(y*z-x*w)],
                     [2*(x*z-y*w), 2*(y*z+x*w), 1-2*(x*x+y*y)]]
    matrix[:3, 3] = position
    return matrix


def _world_matrices(bones, pose=None):
    by_name = {b["name"]: b for b in bones}
    if len(by_name) != len(bones):
        raise ModelError("Duplicate skeleton bone name")
    result, visiting = {}, set()

    def resolve(name):
        if name in result:
            return result[name]
        if name in visiting or name not in by_name:
            raise ModelError("Cyclic or missing skeleton parent")
        visiting.add(name)
        b = by_name[name]
        q, p = (pose or {}).get(name, (b["quaternion"], b["position"]))
        local = _matrix(q, p)
        result[name] = resolve(b["parent"]) @ local if b["parent"] else local
        visiting.remove(name)
        return result[name]

    for name in by_name:
        resolve(name)
    return result


def _pose(animation):
    if not animation["timings"]:
        raise ModelError("Idle animation has no frames")
    # Use an actual recorded idle frame, avoiding invented poses or interpolation rules.
    target = animation["duration"] * 0.2
    frame = min(range(len(animation["timings"])), key=lambda i: abs(animation["timings"][i] - target))
    return {t["bone"]: (t["quaternions"][frame*4:frame*4+4], t["positions"][frame*3:frame*3+3])
            for t in animation["tracks"]}


def _skin(mesh, bind, posed):
    points = np.asarray(mesh["positions"], dtype=np.float64).reshape(-1, 3)
    normals = np.asarray(mesh["normals"], dtype=np.float64).reshape(-1, 3)
    if mesh["bones"]:
        if any(name not in bind for name in mesh["bones"]):
            raise ModelError("Mesh references a missing skeleton bone")
        matrices = np.asarray([posed[name] @ np.linalg.inv(bind[name]) for name in mesh["bones"]])
        joints = np.asarray(mesh["joints"]).reshape(-1, 4)
        weights = np.asarray(mesh["weights"]).reshape(-1, 4)
        transforms = (matrices[joints] * weights[:, :, None, None]).sum(axis=1)
        points = np.einsum("nij,nj->ni", transforms[:, :3, :3], points) + transforms[:, :3, 3]
        normals = np.einsum("nij,nj->ni", transforms[:, :3, :3], normals)
    # Convert the DirectX left-handed frame into a right-handed camera frame.
    points[:, 2] *= -1
    normals[:, 2] *= -1
    lengths = np.linalg.norm(normals, axis=1)
    normals /= np.maximum(lengths[:, None], 1e-12)
    if not np.isfinite(points).all():
        raise ModelError("Invalid posed geometry")
    return points, normals


def _portrait_mesh_indices(meshes):
    """Fit the main mesh cluster without framing props animated far off the body.

    A gap exceeding four times the largest mesh's span is treated as displaced
    geometry. It still participates in rasterization, but cannot shrink the
    portrait's camera fit. This conservative bound keeps ordinary detached parts.
    """
    if not meshes or any(not len(m["positions"]) or not np.isfinite(m["positions"]).all() for m in meshes):
        raise ModelError("Empty or non-finite geometry")
    bounds = [(m["positions"].min(axis=0), m["positions"].max(axis=0)) for m in meshes]
    spans = [float((high - low).max()) for low, high in bounds]
    anchor = max(range(len(meshes)), key=lambda i: (spans[i], len(meshes[i]["positions"])))
    low, high = bounds[anchor]
    return [i for i, (other_low, other_high) in enumerate(bounds)
            if np.maximum(np.maximum(other_low - high, low - other_high), 0).max() <= spans[anchor] * 4]


def rasterize(meshes, *, size=RENDER_SIZE):
    """Orthographic textured triangles with a real depth buffer and 2x antialiasing.

    Mesh dictionaries contain posed positions/normals, UVs, indices, a texture
    array, and an explicit alpha-cutout flag. Texture UVs use the DirectX origin.
    """
    if not 16 <= size <= 1024 or not meshes:
        raise ModelError("Invalid render dimensions or empty model")
    total_vertices = sum(len(m["positions"]) for m in meshes)
    total_triangles = sum(len(m["indices"]) for m in meshes)
    if total_vertices > MAX_MODEL_VERTICES or total_triangles > MAX_MODEL_TRIANGLES:
        raise ModelError("Model exceeds render budget")
    yaw, pitch = -0.3, 0.02
    right = np.array([math.cos(yaw), 0, -math.sin(yaw)])
    forward = np.array([math.sin(yaw)*math.cos(pitch), math.sin(pitch), math.cos(yaw)*math.cos(pitch)])
    up = np.cross(forward, right)
    camera = np.asarray([right, up, forward]).T
    projected = [m["positions"] @ camera for m in meshes]
    all_points = np.concatenate(projected)
    if not len(all_points) or not np.isfinite(all_points).all():
        raise ModelError("Empty or non-finite geometry")
    fit_points = np.concatenate([projected[i] for i in _portrait_mesh_indices(meshes)])
    low, high = fit_points[:, :2].min(axis=0), fit_points[:, :2].max(axis=0)
    extent = float((high-low).max())
    if extent < 1e-8:
        raise ModelError("Degenerate model bounds")
    dimension = size * 2
    scale = dimension * 0.88 / extent
    center = (low+high)/2
    image = np.zeros((dimension, dimension, 4), dtype=np.uint8)
    depth = np.full((dimension, dimension), -np.inf, dtype=np.float64)
    light = np.array([-0.4, 0.7, 0.6]); light /= np.linalg.norm(light)

    for mesh, points in zip(meshes, projected):
        screen = points.copy()
        screen[:, :2] = (screen[:, :2]-center) * scale + dimension/2
        screen[:, 1] = dimension-screen[:, 1]
        texture = mesh["texture"]
        height, width = texture.shape[:2]
        uv = mesh["uvs"]
        lighting = np.clip(0.72 + 0.38 * (mesh["normals"] @ light), 0.42, 1.1)
        for face in mesh["indices"]:
            tri = screen[face]
            x0, y0 = np.maximum(np.floor(tri[:, :2].min(axis=0)).astype(int), 0)
            x1, y1 = np.minimum(np.ceil(tri[:, :2].max(axis=0)).astype(int), dimension-1)
            if x1 < x0 or y1 < y0:
                continue
            a, b, c = tri
            denominator = (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
            if abs(denominator) < 1e-10:
                continue
            xx, yy = np.meshgrid(np.arange(x0,x1+1)+0.5, np.arange(y0,y1+1)+0.5)
            w0 = ((b[1]-c[1])*(xx-c[0])+(c[0]-b[0])*(yy-c[1])) / denominator
            w1 = ((c[1]-a[1])*(xx-c[0])+(a[0]-c[0])*(yy-c[1])) / denominator
            w2 = 1-w0-w1
            z = w0*a[2]+w1*b[2]+w2*c[2]
            region_depth = depth[y0:y1+1, x0:x1+1]
            visible = (w0 >= -1e-9) & (w1 >= -1e-9) & (w2 >= -1e-9) & (z > region_depth)
            if not visible.any():
                continue
            weights = np.stack([w0[visible],w1[visible],w2[visible]], axis=1)
            texcoord = weights @ uv[face]
            # Repeat UVs like the client's diffuse sampler; no vertical flip.
            tx = np.floor(np.mod(texcoord[:, 0],1)*width).astype(int)
            ty = np.floor(np.mod(texcoord[:, 1],1)*height).astype(int)
            samples = texture[ty,tx].copy()
            if mesh["cutout"]:
                accepted = samples[:, 3] >= 102
            else:
                accepted = np.ones(len(samples), dtype=bool)
            samples[:, :3] = np.clip(samples[:, :3] * (weights @ lighting[face])[:, None],0,255).astype(np.uint8)
            samples[:, 3] = 255  # Cutout alpha; other source alpha may encode sheen.
            ys, xs = np.nonzero(visible)
            ys, xs = ys[accepted], xs[accepted]
            image[y0+ys,x0+xs] = samples[accepted]
            region_depth[ys,xs] = z[visible][accepted]
    result = Image.fromarray(image).resize((size,size),Image.Resampling.LANCZOS)
    if result.getchannel("A").getbbox() is None:
        raise ModelError("Model rendered no visible pixels")
    output = io.BytesIO()
    result.save(output,format="PNG",compress_level=6)
    return output.getvalue()


def render_resource(archive, index, resource_path):
    """Return PNG bytes and private provenance for one exact local BSR resource."""
    evidence = []
    bytes_read = 0

    def read(path):
        nonlocal bytes_read
        path = source_path(path)
        entry = index.get(path)
        if entry is None:
            raise ModelError(f"Missing resource dependency: {path}")
        payload = archive.read_payload(entry,limit=16*1024*1024)
        bytes_read += len(payload)
        if bytes_read > 64*1024*1024:
            raise ModelError("Model dependencies exceed read budget")
        evidence.append({"entry": path,"sha256": hashlib.sha256(payload).hexdigest()})
        return payload

    compound_corrections = []
    if resource_path.casefold().endswith(".cpd"):
        resource_paths, compound_corrections = models.compound_resource(read(resource_path))
    else:
        resource_paths = [resource_path]
    meshes, parts, corrected_offsets = [], [], []
    vertex_count = triangle_count = 0
    root_state = None
    for part_path in resource_paths:
        material_sets, mesh_paths, skeleton_path, animation_paths, info = models.resource(read(part_path), details=True)
        # Named artwork depicts the base palette, independent of entity overrides.
        base_materials = [m for m in material_sets if m["id"] == 0]
        if len(base_materials) != 1:
            raise ModelError("Resource has no unambiguous base material palette")
        material_path = base_materials[0]["path"]
        materials = models.material_set(read(material_path))
        source_meshes = [models.mesh(read(path)) for path in mesh_paths]
        used_materials = {m["material"] for m in source_meshes}
        textures = {}
        for material in materials:
            if material["name"] not in used_materials:
                continue
            path = material["texture"]
            if not path:
                color = np.clip(np.asarray(material["diffuse"]) * 255, 0, 255).astype(np.uint8)
                textures[material["name"]] = color.reshape(1,1,4)
                continue
            if not material["absolute"]:
                path = str(PurePosixPath(source_path(material_path)).parent / path.replace("\\", "/"))
            converted = ddj_to_png(read(path))
            if converted.width * converted.height > 4_000_000:
                raise ModelError("Model texture exceeds raster budget")
            with Image.open(io.BytesIO(converted.data)) as texture:
                textures[material["name"]] = np.asarray(texture.convert("RGBA"))
        idle_candidates = [p for p in animation_paths if re.search(r"_stand\d*$", PurePosixPath(p.replace("\\","/")).stem, re.I)]
        idle = idle_candidates[0] if idle_candidates else None
        pose_warning = None
        bones, bind, posed = [], {}, {}
        try:
            if not skeleton_path and root_state is not None:
                bones, bind, posed = root_state
            else:
                bones = models.skeleton(read(skeleton_path)) if skeleton_path else []
                bind = _world_matrices(bones)
                pose = _pose(models.animation(read(idle))) if idle else {}
                pose = {name: transform for name, transform in pose.items() if name in bind}
                posed = _world_matrices(bones,pose)
            if any(name not in bind for mesh in source_meshes for name in mesh["bones"]):
                raise ModelError("Mesh references a missing skeleton bone")
        except ModelError as exc:
            # Rest geometry remains the actual model when custom skeletons fail.
            pose_warning, idle = str(exc), None
            bind, posed = {}, {}
        attachment = None
        if info["attachmentBone"] and root_state is not None:
            root_posed = root_state[2]
            if info["attachmentBone"] not in root_posed:
                raise ModelError("Compound attachment references a missing parent bone")
            reflection = np.diag([1,1,-1,1])
            attachment = reflection @ root_posed[info["attachmentBone"]] @ reflection
        if root_state is None:
            root_state = bones, bind, posed
        for path, mesh in zip(mesh_paths, source_meshes):
            if mesh["faceOffsetDelta"]:
                corrected_offsets.append({"entry": source_path(path), "delta": mesh["faceOffsetDelta"]})
            vertex_count += len(mesh["positions"]) // 3
            triangle_count += len(mesh["indices"]) // 3
            if vertex_count > MAX_MODEL_VERTICES or triangle_count > MAX_MODEL_TRIANGLES:
                raise ModelError("Model exceeds render budget")
            if mesh["material"] not in textures:
                raise ModelError("Mesh references a missing material")
            points, normals = _skin({**mesh,"bones": []} if pose_warning else mesh,bind,posed)
            if attachment is not None:
                points = points @ attachment[:3,:3].T + attachment[:3,3]
                normals = normals @ attachment[:3,:3].T
            meshes.append({"source": source_path(path), "positions": points,"normals": normals,
                           "uvs": np.asarray(mesh["uvs"]).reshape(-1,2),
                           "indices": np.asarray(mesh["indices"],dtype=int).reshape(-1,3),
                           "texture": textures[mesh["material"]],
                           "cutout": mesh["material"].casefold().endswith("_2side")})
        parts.append({"resource": source_path(part_path), "bones": len(bones),
                      "pose": "recorded-stand-frame-20-percent" if idle else "rest-geometry",
                      "poseWarning": pose_warning, "attachmentBone": info["attachmentBone"]})
    fit_indices = set(_portrait_mesh_indices(meshes)) if meshes else set()
    return rasterize(meshes), {"renderer": RENDERER_VERSION,"dependencies": evidence,
                               "cameraFitExcludedMeshes": [m["source"] for i, m in enumerate(meshes) if i not in fit_indices],
                               "materialPalette": 0,"correctedFaceOffsets": corrected_offsets,
                               "correctedCompoundPaths": compound_corrections,"parts": parts,
                               "pose": parts[0]["pose"],"poseWarning": parts[0]["poseWarning"],
                               "meshes": len(meshes),"bones": parts[0]["bones"],
                               "triangles": triangle_count}
