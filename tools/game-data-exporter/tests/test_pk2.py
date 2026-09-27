from __future__ import annotations

import struct

import pytest

from phmon_game_exporter.pk2 import ENTRY_SIZE, HEADER_SIZE, PK2Archive, PK2Error
import phmon_game_exporter.pk2 as pk2_module
from .helpers import TEST_KEY, make_pk2


def test_reads_fixture_directory_and_payload(tmp_path):
    path = tmp_path / "small.pk2"
    make_pk2(path, name="A.ddj", payload=b"payload")
    with PK2Archive(path, key=TEST_KEY) as archive:
        info = archive.inventory()
        assert info.encrypted and info.checksum_valid
        assert [(entry.path, entry.size) for entry in info.entries] == [("A.ddj", 7)]
        assert archive.read_payload(info.entries[0]) == b"payload"


@pytest.mark.parametrize("name", ["../outside.ddj", r"..\outside.ddj", r"C:\outside.ddj", r"\\host\share\a"])
def test_rejects_unsafe_entry_names(tmp_path, name):
    path = tmp_path / "unsafe.pk2"
    make_pk2(path, name=name)
    with PK2Archive(path, key=TEST_KEY) as archive, pytest.raises(PK2Error):
        archive.inventory()


def test_rejects_cyclic_root_block_chain(tmp_path):
    path = tmp_path / "cycle.pk2"
    make_pk2(path, name=None, next_block=HEADER_SIZE)
    with PK2Archive(path, key=TEST_KEY) as archive, pytest.raises(PK2Error, match="cyclic"):
        archive.inventory()


@pytest.mark.parametrize(
    ("limit_name", "limit_value"),
    [("MAX_DIRECTORY_BLOCKS", 1), ("MAX_DIRECTORY_BYTES", pk2_module.BLOCK_SIZE)],
)
def test_rejects_directory_chain_over_block_or_byte_limit(tmp_path, monkeypatch, limit_name, limit_value):
    path = tmp_path / "long-directory-chain.pk2"
    second_block = HEADER_SIZE + pk2_module.BLOCK_SIZE
    make_pk2(path, name=None, next_block=second_block)
    with path.open("ab") as stream:
        stream.write(b"\0" * pk2_module.BLOCK_SIZE)
    monkeypatch.setattr(pk2_module, limit_name, limit_value)

    with PK2Archive(path, key=TEST_KEY) as archive, pytest.raises(PK2Error, match="directory block/byte limit"):
        archive.inventory()


def test_rejects_truncated_payload(tmp_path):
    path = tmp_path / "truncated.pk2"
    make_pk2(path, payload=b"x", entry_size=9)
    with PK2Archive(path, key=TEST_KEY) as archive, pytest.raises(PK2Error, match="range"):
        archive.inventory()


def test_supports_authored_sparse_payload_above_four_gibibytes(tmp_path):
    path = tmp_path / "sparse-large-offset.pk2"
    offset = 0x1_0000_1234
    try:
        make_pk2(path, payload=b"above-4g", entry_position=offset, sparse=True)
    except (AttributeError, OSError) as exc:
        pytest.skip(f"filesystem does not support sparse files: {exc}")
    with PK2Archive(path, key=TEST_KEY) as archive:
        entry = archive.inventory().entries[0]
        assert entry.position == offset
        assert archive.read_payload(entry) == b"above-4g"


def test_entry_layout_keeps_unsigned_64_bit_offset():
    offset = 0xFEDC_BA98_7654_3210
    raw = bytearray(ENTRY_SIZE)
    raw[0] = 2
    raw[1:5] = b"a.dd"
    struct.pack_into("<Q", raw, 106, offset)
    struct.pack_into("<I", raw, 114, 1)
    assert PK2Archive._parse_entry(bytes(raw), "", encoding="ascii").position == offset
