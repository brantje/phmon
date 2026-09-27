from __future__ import annotations

import io

import pytest
from PIL import Image

from phmon_game_exporter.textures import TextureError, ddj_to_png
from .helpers import ddj_rgba


def test_decodes_ddj_dds_and_preserves_alpha_and_dimensions():
    result = ddj_to_png(ddj_rgba((200, 80, 30, 90)))
    assert (result.width, result.height) == (1, 1)
    assert result.media_type == "image/png"
    with Image.open(io.BytesIO(result.data)) as image:
        assert image.size == (1, 1)
        assert image.getpixel((0, 0)) == (200, 80, 30, 90)


def test_decodes_observed_big_endian_ddj_size_variant():
    result = ddj_to_png(ddj_rgba(size_endian="big"))
    assert (result.width, result.height) == (1, 1)
    assert result.wrapper_field_status == "big-endian-length"


def test_decodes_unmatched_wrapper_field_only_after_full_dds_decode():
    payload = bytearray(ddj_rgba())
    payload[12:16] = (1784).to_bytes(4, "little")
    result = ddj_to_png(bytes(payload))
    assert result.wrapper_field_status == "unmatched-advisory-field"


def test_decodes_png_embedded_in_ddj_wrapper():
    source = Image.new("RGBA", (2, 3), (10, 20, 30, 40))
    stream = io.BytesIO()
    source.save(stream, format="PNG")
    embedded = stream.getvalue()
    payload = b"JMXVDDJ 1000" + (len(embedded) + 8).to_bytes(4, "little") + b"\x03\0\0\0" + embedded
    result = ddj_to_png(payload)
    assert result.embedded_format == "PNG"
    assert (result.width, result.height) == (2, 3)


def test_decodes_palette_png_transparency_embedded_in_ddj_wrapper():
    source = Image.new("P", (1, 1), 0)
    source.putpalette([255, 0, 0, 0, 0, 255] + [0] * (768 - 6))
    stream = io.BytesIO()
    source.save(stream, format="PNG", transparency=0)
    embedded = stream.getvalue()
    payload = b"JMXVDDJ 1000" + (len(embedded) + 8).to_bytes(4, "little") + b"\x03\0\0\0" + embedded

    result = ddj_to_png(payload)

    with Image.open(io.BytesIO(result.data)) as image:
        assert image.mode == "RGBA"
        assert image.getpixel((0, 0)) == (255, 0, 0, 0)


@pytest.mark.parametrize(
    "payload",
    [b"", b"JMXVDDJ 1000" + b"\0" * 8 + b"\x03\0\0\0", b"JMXVDDJ 1000" + b"\0" * 4 + b"\x02\0\0\0DDS "],
)
def test_rejects_malformed_ddj(payload):
    with pytest.raises(TextureError):
        ddj_to_png(payload)
