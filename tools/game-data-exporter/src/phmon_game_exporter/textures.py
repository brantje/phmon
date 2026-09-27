"""Verified DDJ-to-PNG conversion used by the standalone exporter."""

from __future__ import annotations

import io
import struct
from dataclasses import dataclass

from PIL import Image, UnidentifiedImageError

DDJ_HEADER = b"JMXVDDJ 1000"
DDJ_HEADER_SIZE = 20
MAX_IMAGE_EDGE = 8192
MAX_IMAGE_PIXELS = 32_000_000


class TextureError(ValueError):
    """A malformed or unsupported image payload."""


@dataclass(frozen=True, slots=True)
class PNGImage:
    data: bytes
    width: int
    height: int
    wrapper_field_status: str
    wrapper_field_hex: str
    embedded_format: str
    media_type: str = "image/png"


def ddj_to_png(payload: bytes) -> PNGImage:
    """Validate the DDJ wrapper, decode its DDS payload, and emit metadata-free PNG."""
    if len(payload) < DDJ_HEADER_SIZE or payload[: len(DDJ_HEADER)] != DDJ_HEADER:
        raise TextureError("unsupported DDJ header")
    size_bytes = payload[12:16]
    expected_size = len(payload) - 12
    little_size = struct.unpack("<I", size_bytes)[0]
    big_size = struct.unpack(">I", size_bytes)[0]
    if little_size == expected_size:
        field_status = "little-endian-length"
    elif big_size == expected_size:
        field_status = "big-endian-length"
    else:
        # Some files have this field populated differently. Do not use it to
        # determine bounds: the actual PK2 entry length and decoded DDS are bounded.
        field_status = "unmatched-advisory-field"
    if payload[16] != 3 or payload[17:20] != b"\0\0\0":
        raise TextureError("unsupported DDJ wrapper version or flags")
    dds = payload[DDJ_HEADER_SIZE:]
    if dds.startswith(b"DDS "):
        expected_format = "DDS"
    elif dds.startswith(b"\x89PNG\r\n\x1a\n"):
        expected_format = "PNG"
    else:
        raise TextureError("DDJ does not contain a supported PNG or DDS image")
    try:
        with Image.open(io.BytesIO(dds)) as decoded:
            if decoded.format != expected_format:
                raise TextureError("embedded image signature and decoder format do not agree")
            width, height = decoded.size
            if (
                width < 1
                or height < 1
                or width > MAX_IMAGE_EDGE
                or height > MAX_IMAGE_EDGE
                or width * height > MAX_IMAGE_PIXELS
            ):
                raise TextureError(f"image dimensions exceed limits: {width}x{height}")
            decoded.load()
            has_transparency = "A" in decoded.getbands() or "transparency" in decoded.info
            converted = decoded.convert("RGBA" if has_transparency else "RGB")
    except (OSError, UnidentifiedImageError, Image.DecompressionBombError) as exc:
        raise TextureError(f"DDS decoding failed: {exc}") from exc
    output = io.BytesIO()
    converted.save(output, format="PNG", optimize=False, compress_level=6)
    return PNGImage(output.getvalue(), width, height, field_status, size_bytes.hex(), expected_format)
