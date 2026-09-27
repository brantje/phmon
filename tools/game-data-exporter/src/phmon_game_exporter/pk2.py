"""Bounded, read-only PK2 inventory and random access.

The format fields follow the public Silkroad PK2 layout cited in format-notes.md.
Source entry names are never used as output paths.
"""

from __future__ import annotations

import hashlib
import os
import struct
from dataclasses import dataclass
from pathlib import Path, PurePosixPath, PureWindowsPath
from typing import BinaryIO

from Crypto.Cipher import Blowfish

SIGNATURE = b"JoyMax File Manager!\n\0\0\0\0\0\0\0\0\0"
HEADER_SIZE = 256
HEADER_VERSION = 0x01000002
BLOCK_ENTRIES = 20
ENTRY_SIZE = 128
BLOCK_SIZE = BLOCK_ENTRIES * ENTRY_SIZE
MAX_ENTRIES = 2_000_000
MAX_DIRECTORY_DEPTH = 128
MAX_DIRECTORY_BLOCKS = 100_000
MAX_DIRECTORY_BYTES = 256 * 1024 * 1024
MAX_ENTRY_BYTES = 512 * 1024 * 1024
SALT = bytes((0x03, 0xF8, 0xE4, 0x44, 0x88, 0x99, 0x3F, 0x64, 0xFE, 0x35))
CHECKSUM = b"Joymax Pak File\0"


class PK2Error(ValueError):
    """A malformed, unsupported, or unsafe archive condition."""


@dataclass(frozen=True, slots=True)
class Entry:
    kind: int
    name: str
    path: str
    position: int
    size: int
    next_block: int


@dataclass(frozen=True, slots=True)
class ArchiveInfo:
    path: str
    size: int
    encrypted: bool
    checksum_valid: bool
    entry_count: int
    directory_count: int
    file_count: int
    extension_counts: dict[str, int]
    entries: tuple[Entry, ...]


def _pk2_key(key: str) -> bytes:
    raw = key.encode("ascii", "strict")
    if not 4 <= len(raw) <= 56:
        raise PK2Error("PK2 key must contain 4 to 56 ASCII bytes")
    return bytes(value ^ SALT[i] if i < len(SALT) else value for i, value in enumerate(raw))


class _Cipher:
    """PK2's little-endian Blowfish block convention over PyCryptodome."""

    def __init__(self, key: str):
        self._cipher = Blowfish.new(_pk2_key(key), Blowfish.MODE_ECB)

    def transform(self, data: bytes, *, decrypt: bool) -> bytes:
        if len(data) % 8:
            raise PK2Error("encrypted PK2 data is not 8-byte aligned")
        # The PK2 implementation reads each 32-bit half as little-endian.
        swapped = bytearray(data)
        for start in range(0, len(swapped), 4):
            swapped[start : start + 4] = swapped[start : start + 4][::-1]
        result = (self._cipher.decrypt if decrypt else self._cipher.encrypt)(bytes(swapped))
        output = bytearray(result)
        for start in range(0, len(output), 4):
            output[start : start + 4] = output[start : start + 4][::-1]
        return bytes(output)


def _safe_name(raw: bytes, path: str, *, encoding: str) -> str:
    if b"\0" in raw:
        raw = raw.split(b"\0", 1)[0]
    try:
        name = raw.decode(encoding, errors="strict")
    except (UnicodeDecodeError, LookupError) as exc:
        raise PK2Error(f"invalid {encoding} entry name at {path!r}") from exc
    if name in ("", ".", ".."):
        if name in (".", ".."):
            return name
        raise PK2Error(f"empty entry name at {path!r}")
    if any(ord(char) < 32 for char in name):
        raise PK2Error(f"control character in PK2 entry name at {path!r}")
    windows = PureWindowsPath(name)
    posix = PurePosixPath(name.replace("\\", "/"))
    if (
        windows.is_absolute()
        or windows.drive
        or posix.is_absolute()
        or any(part in (".", "..") for part in posix.parts)
        or ":" in name
    ):
        raise PK2Error(f"unsafe path component in PK2 entry {name!r}")
    return name


class PK2Archive:
    """Read-only stream parser. File reads use 64-bit offsets and bounded sizes."""

    def __init__(self, path: str | os.PathLike[str], *, key: str = "169841"):
        self.path = Path(path)
        self.key = key
        self._stream: BinaryIO | None = None
        self._cipher: _Cipher | None = None
        self._size = 0
        self.encrypted = False
        self.checksum_valid = False

    def __enter__(self) -> PK2Archive:
        self._stream = self.path.open("rb")
        self._stream.seek(0, os.SEEK_END)
        self._size = self._stream.tell()
        if self._size < HEADER_SIZE + BLOCK_SIZE:
            self.close()
            raise PK2Error("archive is shorter than its header and root block")
        self._stream.seek(0)
        header = self._stream.read(HEADER_SIZE)
        if len(header) != HEADER_SIZE or header[:30] != SIGNATURE:
            self.close()
            raise PK2Error("invalid PK2 signature or truncated header")
        version = struct.unpack_from("<I", header, 30)[0]
        if version != HEADER_VERSION:
            self.close()
            raise PK2Error(f"unsupported PK2 version 0x{version:08x}")
        self.encrypted = bool(header[34])
        self._cipher = _Cipher(self.key)
        expected = self._cipher.transform(CHECKSUM, decrypt=False)[:3]
        self.checksum_valid = expected == header[35:38]
        if self.encrypted and not self.checksum_valid:
            # Some private clients keep a patched checksum. The root block still
            # has to prove the key by decoding to structurally valid entries.
            try:
                self._read_block(HEADER_SIZE)
            except PK2Error as exc:
                self.close()
                raise PK2Error("archive key/header checksum did not validate") from exc
        return self

    def close(self) -> None:
        if self._stream is not None:
            self._stream.close()
            self._stream = None

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()

    def _require_open(self) -> BinaryIO:
        if self._stream is None:
            raise PK2Error("archive is not open")
        return self._stream

    def _checked_range(self, offset: int, size: int) -> None:
        if offset < HEADER_SIZE or size < 0 or offset > self._size or size > self._size - offset:
            raise PK2Error(f"invalid archive range ({offset}, {size})")

    def _read_at(self, offset: int, size: int, *, limit: int | None = None) -> bytes:
        self._checked_range(offset, size)
        if limit is not None and size > limit:
            raise PK2Error(f"archive entry exceeds configured read limit ({size} bytes)")
        stream = self._require_open()
        stream.seek(offset, os.SEEK_SET)
        result = stream.read(size)
        if len(result) != size:
            raise PK2Error("truncated PK2 entry payload")
        return result

    def read_payload(self, entry: Entry, *, limit: int = MAX_ENTRY_BYTES) -> bytes:
        if entry.kind != 2:
            raise PK2Error("cannot read payload of a directory")
        return self._read_at(entry.position, entry.size, limit=limit)

    def _read_block(self, offset: int) -> bytes:
        self._checked_range(offset, BLOCK_SIZE)
        data = self._read_at(offset, BLOCK_SIZE)
        if self.encrypted:
            assert self._cipher is not None
            data = self._cipher.transform(data, decrypt=True)
        return data

    @staticmethod
    def _parse_entry(raw: bytes, parent: str, *, encoding: str) -> Entry:
        if len(raw) != ENTRY_SIZE:
            raise PK2Error("invalid PK2 directory entry size")
        kind = raw[0]
        if kind == 0:
            return Entry(0, "", parent, 0, 0, struct.unpack_from("<Q", raw, 118)[0])
        if kind not in (1, 2):
            raise PK2Error(f"unsupported PK2 entry type {kind}")
        name = _safe_name(raw[1:82], parent, encoding=encoding)
        position = struct.unpack_from("<Q", raw, 106)[0]
        size = struct.unpack_from("<I", raw, 114)[0]
        next_block = struct.unpack_from("<Q", raw, 118)[0]
        if position == 0:
            raise PK2Error(f"zero data/directory offset at {parent!r}")
        path = name if not parent else f"{parent}/{name}"
        return Entry(kind, name, path, position, size, next_block)

    def inventory(self, *, encoding: str = "euc_kr") -> ArchiveInfo:
        self._require_open()
        entries: list[Entry] = []
        seen_directories: set[int] = set()
        directory_blocks = 0
        stack: list[tuple[str, int, int]] = [("", HEADER_SIZE, 0)]
        while stack:
            parent, start, depth = stack.pop()
            if depth > MAX_DIRECTORY_DEPTH:
                raise PK2Error("maximum PK2 directory depth exceeded")
            if start in seen_directories:
                raise PK2Error(f"repeated/cyclic directory chain offset {start}")
            seen_directories.add(start)
            chain_offsets: set[int] = set()
            offset = start
            while offset:
                if offset in chain_offsets:
                    raise PK2Error(f"cyclic PK2 block chain at offset {offset}")
                chain_offsets.add(offset)
                directory_blocks += 1
                if directory_blocks > MAX_DIRECTORY_BLOCKS or directory_blocks * BLOCK_SIZE > MAX_DIRECTORY_BYTES:
                    raise PK2Error("maximum PK2 directory block/byte limit exceeded")
                block = self._read_block(offset)
                block_entries: list[Entry] = []
                for i in range(BLOCK_ENTRIES):
                    raw = block[i * ENTRY_SIZE : (i + 1) * ENTRY_SIZE]
                    item = self._parse_entry(raw, parent, encoding=encoding)
                    block_entries.append(item)
                    if item.kind and item.name not in (".", ".."):
                        entries.append(item)
                        if len(entries) > MAX_ENTRIES:
                            raise PK2Error("maximum PK2 entry count exceeded")
                        if item.kind == 1:
                            self._checked_range(item.position, BLOCK_SIZE)
                            stack.append((item.path, item.position, depth + 1))
                        else:
                            self._checked_range(item.position, item.size)
                offset = block_entries[-1].next_block
                if offset:
                    self._checked_range(offset, BLOCK_SIZE)
        entries.sort(key=lambda item: (item.path.casefold(), item.path))
        counts: dict[str, int] = {}
        for item in entries:
            if item.kind != 2:
                continue
            suffix = Path(item.name).suffix.casefold() or "[no extension]"
            counts[suffix] = counts.get(suffix, 0) + 1
        files = sum(item.kind == 2 for item in entries)
        directories = len(entries) - files
        return ArchiveInfo(
            str(self.path.resolve()),
            self._size,
            self.encrypted,
            self.checksum_valid,
            len(entries),
            directories,
            files,
            dict(sorted(counts.items())),
            tuple(entries),
        )


def sha256_file(path: str | os.PathLike[str], *, chunk_size: int = 1024 * 1024) -> str:
    digest = hashlib.sha256()
    with Path(path).open("rb") as stream:
        while chunk := stream.read(chunk_size):
            digest.update(chunk)
    return digest.hexdigest()
