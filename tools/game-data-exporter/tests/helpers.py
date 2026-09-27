from __future__ import annotations

import os
import struct
from pathlib import Path

from phmon_game_exporter.pk2 import (
    BLOCK_ENTRIES,
    BLOCK_SIZE,
    CHECKSUM,
    ENTRY_SIZE,
    HEADER_SIZE,
    HEADER_VERSION,
    SIGNATURE,
    _Cipher,
)

TEST_KEY = "169841"


def ddj_rgba(rgba: tuple[int, int, int, int] = (220, 30, 40, 255), *, size_endian: str = "little") -> bytes:
    red, green, blue, alpha = rgba
    pixel_format = struct.pack(
        "<8I",
        32,
        0x41,
        0,
        32,
        0x00FF0000,
        0x0000FF00,
        0x000000FF,
        0xFF000000,
    )
    header_words = [124, 0x100F, 1, 1, 4, 0, 0, *([0] * 11)]
    dds_header = struct.pack("<7I11I", *header_words) + pixel_format + struct.pack("<5I", 0x1000, 0, 0, 0, 0)
    dds = b"DDS " + dds_header + bytes((blue, green, red, alpha))
    wrapper = b"JMXVDDJ 1000" + (len(dds) + 8).to_bytes(4, size_endian) + b"\x03\0\0\0"
    return wrapper + dds


def make_pk2(
    path: Path,
    *,
    name: str | None = "sample.bin",
    payload: bytes = b"x",
    entry_position: int | None = None,
    entry_size: int | None = None,
    next_block: int = 0,
    sparse: bool = False,
) -> int:
    cipher = _Cipher(TEST_KEY)
    block = bytearray(BLOCK_SIZE)
    payload_offset = HEADER_SIZE + BLOCK_SIZE if entry_position is None else entry_position
    if name is not None:
        row = bytearray(ENTRY_SIZE)
        row[0] = 2
        encoded = name.encode("euc_kr")
        if len(encoded) > 80:
            raise ValueError("fixture name too long")
        row[1 : 1 + len(encoded)] = encoded
        struct.pack_into("<Q", row, 106, payload_offset)
        struct.pack_into("<I", row, 114, len(payload) if entry_size is None else entry_size)
        block[:ENTRY_SIZE] = row
    struct.pack_into("<Q", block, (BLOCK_ENTRIES - 1) * ENTRY_SIZE + 118, next_block)
    header = bytearray(HEADER_SIZE)
    header[:30] = SIGNATURE
    struct.pack_into("<I", header, 30, HEADER_VERSION)
    header[34] = 1
    header[35:38] = cipher.transform(CHECKSUM, decrypt=False)[:3]
    with path.open("wb") as stream:
        stream.write(header)
        stream.write(cipher.transform(bytes(block), decrypt=False))
    if sparse:
        if os.name == "nt":
            import ctypes

            kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
            handle = kernel32.CreateFileW(str(path), 0x40000000, 0x3, None, 3, 0x80, None)
            if handle == ctypes.c_void_p(-1).value:
                raise OSError(ctypes.get_last_error(), "CreateFileW failed for sparse fixture")
            returned = ctypes.c_ulong()
            ok = kernel32.DeviceIoControl(handle, 0x900C4, None, 0, None, 0, ctypes.byref(returned), None)
            kernel32.CloseHandle(handle)
            if not ok:
                raise OSError(ctypes.get_last_error(), "FSCTL_SET_SPARSE failed")
    if name is not None:
        with path.open("r+b") as stream:
            stream.seek(payload_offset)
            stream.write(payload)
        if sparse and os.name != "nt":
            stats = path.stat()
            allocated = getattr(stats, "st_blocks", None)
            if allocated is not None and allocated * 512 >= stats.st_size:
                raise OSError("filesystem allocated the complete sparse fixture")
    return payload_offset
