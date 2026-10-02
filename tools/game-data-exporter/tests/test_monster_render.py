from __future__ import annotations

import io
import json
import shutil
import struct
from pathlib import Path

import numpy as np
import pytest
from PIL import Image

from phmon_game_exporter import exporter, models
from phmon_game_exporter.cli import validate_bundle
from phmon_game_exporter.monster_render import _world_matrices, rasterize, source_path
from phmon_game_exporter.pk2 import ArchiveInfo, Entry
from phmon_game_exporter.public_assets import materialize_public_assets, validate_public_assets
from .helpers import ddj_rgba
from .test_exporter import _FakeArchive, _make_sources, _text


def _square(color, z=0, *, cutout=False):
    texture = np.full((4,4,4), color, dtype=np.uint8)
    return {"positions": np.array([[-1,1,z],[-1,-1,z],[1,-1,z],[1,1,z]],dtype=float),
            "normals": np.tile([0,0,1],(4,1)),"uvs": np.array([[0,0],[0,1],[1,1],[1,0]],dtype=float),
            "indices": np.array([[0,1,2],[0,2,3]]),"texture": texture,"cutout":cutout}


def _image(meshes):
    return Image.open(io.BytesIO(rasterize(meshes,size=64)))


def test_depth_occlusion_is_independent_of_mesh_order():
    near = _square((0,255,0,255),z=0.2)
    far = _square((255,0,0,255),z=-0.2)
    a, b = _image([near,far]), _image([far,near])
    assert a.tobytes() == b.tobytes()
    assert a.getpixel((32,32))[1] > 200
    assert a.getpixel((0,0))[3] == 0


def test_transparent_cutout_does_not_occlude_geometry_behind_it():
    cutout = _square((0,255,0,0),z=0.2,cutout=True)
    behind = _square((255,0,0,255),z=-0.2)
    assert _image([cutout,behind]).getpixel((32,32))[0] > 200
    # Non-cutout diffuse alpha can encode sheen, rather than invisibility.
    cutout["cutout"] = False
    assert _image([cutout,behind]).getpixel((32,32))[1] > 200


def test_texture_v_origin_is_top_left_and_output_is_deterministic():
    mesh = _square((255,0,0,255))
    mesh["texture"][:2] = [0,0,255,255]
    image = _image([mesh])
    assert image.getpixel((32,16))[2] > image.getpixel((32,16))[0]
    assert image.getpixel((32,48))[0] > image.getpixel((32,48))[2]
    assert rasterize([mesh],size=64) == rasterize([mesh],size=64)
    assert image.mode == "RGBA" and image.size == (64,64)


@pytest.mark.parametrize("path", ["../bad.bsr","/root/bad.bsr",r"C:\bad.bsr","res/mob/../bad.bsr","res//bad.bsr"])
def test_rejects_unsafe_archive_resource_paths(path):
    with pytest.raises(models.ModelError):
        source_path(path)


def test_rejects_cycles_truncation_and_excessive_counts():
    bones=[{"name":"a","parent":"b","quaternion":[0,0,0,1],"position":[0,0,0]},
           {"name":"b","parent":"a","quaternion":[0,0,0,1],"position":[0,0,0]}]
    with pytest.raises(models.ModelError,match="Cyclic"):
        _world_matrices(bones)
    with pytest.raises(models.ModelError,match="Truncated"):
        models.mesh(b"JMXVBMS 0110")
    with pytest.raises(models.ModelError,match="limit"):
        models.Reader(b"JMXVBMS 0110"+struct.pack("<I",2**32-1),b"JMXVBMS 0110").count()


def _string(value):
    encoded=value.encode("ascii")
    return struct.pack("<I",len(encoded))+encoded


def _authored_resources():
    """Minimal original triangle in documented layouts; no game assets in tests."""
    material = b"JMXVBMT 0102"+struct.pack("<I",1)+_string("body")
    material += struct.pack("<16ffI",*([1.0]*16),0.0,0x300)+_string("fixture.ddj")+struct.pack("<f3B",1.0,0,0,0)
    mesh_header=b"JMXVBMS 0110"+bytes(48)+struct.pack("<3I",1,0,0)+_string("triangle")+_string("body")+struct.pack("<I",0)
    vertices=struct.pack("<I",3)
    for p, uv in [((-1,-1,0),(0,1)),((1,-1,0),(1,1)),((0,1,0),(0.5,0))]:
        vertices+=struct.pack("<8ffII",*p,0,0,1,*uv,0.0,0,0)
    skin=struct.pack("<I",0)
    faces=struct.pack("<I3H",1,0,1,2)
    mesh=bytearray(mesh_header+vertices+skin+faces)
    struct.pack_into("<3I",mesh,12,len(mesh_header),len(mesh_header)+len(vertices),len(mesh_header)+len(vertices)+len(skin))
    mat_section=struct.pack("<2I",1,0)+_string("prim/mtrl/fixture.bmt")
    mesh_section=struct.pack("<I",1)+_string("prim/mesh/fixture.bms")
    skel_section=struct.pack("<I",0)
    ani_section=struct.pack("<3I",0x1000,0,0)
    resource=bytearray(b"JMXVRES 0109"+bytes(52)+mat_section+mesh_section+skel_section+ani_section)
    offset=64
    for i,section in enumerate([mat_section,mesh_section,skel_section,ani_section]):
        struct.pack_into("<I",resource,12+i*4,offset)
        offset+=len(section)
    return {"res/mob/test/fixture.bsr":bytes(resource),"prim/mesh/fixture.bms":bytes(mesh),
            "prim/mtrl/fixture.bmt":material,"prim/mtrl/fixture.ddj":ddj_rgba((210,80,40,255))}


def _monster_source(tmp_path,monkeypatch,resources=None):
    source=_make_sources(tmp_path,monkeypatch)
    (source/"Data.pk2").write_bytes(b"authored model archive marker")
    resources=resources if resources is not None else _authored_resources()
    data_entries=tuple(Entry(2,p.rsplit("/",1)[-1],p,0,len(b),0) for p,b in resources.items())
    _FakeArchive.payloads.update(resources)
    fields=["0"]*55
    fields[0],fields[1],fields[2],fields[52]="1","42","MOB_TEST_FIXTURE","mob/test/fixture.bsr"
    _FakeArchive.payloads["server_dep/silkroad/textdata/characterdata_5000.txt"]=_text("\t".join(fields))

    class Archive(_FakeArchive):
        def inventory(self):
            if self.path.name=="Data.pk2":
                return ArchiveInfo(str(self.path),self.path.stat().st_size,True,True,len(data_entries),0,len(data_entries),{},data_entries)
            return super().inventory()

        def read_payload(self,entry,**kwargs):
            return self.payloads[entry.path.casefold()]

    monkeypatch.setattr(exporter,"PK2Archive",Archive)
    return source


def test_normal_export_renders_named_alias_and_copied_artifacts_validate_without_sources(tmp_path,monkeypatch):
    source=_monster_source(tmp_path,monkeypatch)
    public=tmp_path/"game-assets"
    first=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public,monster_models=["fixture"])
    bundle=Path(first["bundlePath"])
    assert first["monsterRenderCount"]==1
    assert first["monsterRenderUnsupportedCount"]==0
    assert (public/"monsters/fixture.png").is_file()
    assert not (public/"monsters/42.png").exists()
    with Image.open(public/"monsters/fixture.png") as image:
        assert image.size==(512,512) and image.mode=="RGBA"
        assert image.getchannel("A").getextrema()==(0,255)
    catalog=json.loads((bundle/"catalogs/monsters.json").read_text())
    assert catalog["records"][0]["referenceIds"]==[42]
    assert catalog["records"][0]["publicAlias"]=="monsters/fixture.png"
    assert "res/mob" not in (bundle/"catalogs/monsters.json").read_text()
    again=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public,monster_models=["fixture"])
    assert again["identicalBundleReused"]
    selected_all=exporter.export_dataset(source,tmp_path/"exports","fixture-key")
    assert selected_all["datasetId"] != first["datasetId"]
    assert selected_all["monsterRenderCount"]==1
    copied=tmp_path/"copied"
    shutil.copytree(bundle,copied)
    shutil.rmtree(source)
    validate_bundle(copied)
    validate_public_assets(public)


def test_missing_dependency_is_audited_and_never_publishes_a_placeholder(tmp_path,monkeypatch):
    source=_monster_source(tmp_path,monkeypatch)
    _FakeArchive.payloads["res/mob/test/fixture.bsr"]=b"unsupported format"
    public=tmp_path/"game-assets"
    result=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public)
    assert result["monsterRenderCount"]==0
    assert result["monsterRenderUnsupportedCount"]==1
    assert not (public/"monsters/fixture.png").exists()
    audit=json.loads((Path(result["auditPath"])/"tables/monster-renders.json").read_text())
    assert audit[0]["status"]=="unsupported" and "signature" in audit[0]["reason"]
    validate_public_assets(public)


def test_public_alias_rejects_rendered_path_traversal_without_replacing_existing_files(tmp_path,monkeypatch):
    source=_monster_source(tmp_path,monkeypatch)
    public=tmp_path/"game-assets"
    result=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public)
    original=(public/"asset-index.json").read_bytes()
    audit=Path(result["auditPath"])
    rows=json.loads((audit/"assets.json").read_text())
    next(row for row in rows if row["status"]=="rendered")["publicAlias"]="monsters/../../escape.png"
    (audit/"assets.json").write_text(json.dumps(rows))
    with pytest.raises(ValueError,match="alias"):
        materialize_public_assets(Path(result["bundlePath"]),audit,public)
    assert (public/"asset-index.json").read_bytes()==original
    assert not (tmp_path/"escape.png").exists()


def test_selected_model_publication_preserves_existing_icon_bytes_keys_and_dataset(tmp_path,monkeypatch):
    source=_monster_source(tmp_path,monkeypatch)
    public=tmp_path/"game-assets"
    first=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public)
    original=(public/"icon/item/test_blade.png").read_bytes()
    old=json.loads((public/"asset-index.json").read_text())
    old_row=next(row for row in old["files"] if row["path"]=="icon/item/test_blade.png")
    # A fresh source version has a different unrelated icon. Narrow publication
    # must preserve the operator's current icon and catalog semantic keys.
    _FakeArchive.payloads["icon/item/test_blade.ddj"]=ddj_rgba((0,255,0,255))
    (source/"Media.pk2").write_bytes(b"new fixture media archive revision")
    selected=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public,monster_models=["fixture"])
    assert selected["datasetId"]!=first["datasetId"]
    assert (public/"icon/item/test_blade.png").read_bytes()==original
    new=json.loads((public/"asset-index.json").read_text())
    assert next(row for row in new["files"] if row["path"]==old_row["path"])==old_row
    assert new["datasetId"]==old["datasetId"]
    assert new["monsterDatasetId"]==selected["datasetId"]
    validate_public_assets(public)
    before=(public/"asset-index.json").read_bytes()
    _FakeArchive.payloads["res/mob/test/fixture.bsr"]=b"unsupported resource"
    (source/"Data.pk2").write_bytes(b"new unsupported fixture revision")
    failed=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public,monster_models=["fixture"])
    assert failed["monsterRenderUnsupportedCount"]==1
    assert (public/"asset-index.json").read_bytes()==before


def test_data_archive_and_model_selection_both_participate_in_dataset_identity(tmp_path,monkeypatch):
    source=_monster_source(tmp_path,monkeypatch)
    first=exporter.export_dataset(source,tmp_path/"exports","fixture-key",monster_models=["fixture"])
    (source/"Data.pk2").write_bytes(b"authored model archive revision two")
    second=exporter.export_dataset(source,tmp_path/"exports","fixture-key",monster_models=["fixture"])
    assert second["datasetId"]!=first["datasetId"]
    with pytest.raises(models.ModelError,match="Unknown"):
        exporter.export_dataset(source,tmp_path/"exports","fixture-key",monster_models=["no_such_model"])
    with pytest.raises(exporter.ExportError,match="unsafe"):
        exporter.export_dataset(source,tmp_path/"exports","fixture-key",monster_models=["../../escape"])


def test_unique_batch_selects_both_ranks_resolves_base_and_preserves_other_assets(tmp_path,monkeypatch):
    source=_monster_source(tmp_path,monkeypatch)
    path="server_dep/silkroad/textdata/characterdata_5000.txt"
    fields=["0"]*55
    fields[0],fields[1],fields[2],fields[15],fields[52]="1","42","MOB_BASE","3","mob/test/fixture.bsr"
    variant=list(fields)
    variant[1],variant[2],variant[4],variant[15],variant[52]="43","MOB_VARIANT","MOB_BASE","8","xxx"
    elite=list(fields)
    elite[1],elite[2],elite[15]="44","MOB_ELITE","6"
    disabled=list(fields)
    disabled[0],disabled[1],disabled[2]="0","45","MOB_DISABLED"
    _FakeArchive.payloads[path]=_text("\n".join("\t".join(r) for r in [fields,variant,elite,disabled]))
    public=tmp_path/"game-assets"
    initial=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public)
    old_index=json.loads((public/"asset-index.json").read_text())
    result=exporter.export_dataset(source,tmp_path/"exports","fixture-key",public,unique_monsters=True)
    assert result["datasetId"]!=initial["datasetId"]
    catalog=json.loads((Path(result["bundlePath"])/"catalogs/monsters.json").read_text())
    assert catalog["coverage"]["uniqueOnly"] is True
    assert catalog["records"][0]["referenceIds"]==[42,43]
    new_index=json.loads((public/"asset-index.json").read_text())
    assert new_index["datasetId"]==old_index["datasetId"]
    assert next(r for r in new_index["files"] if r["path"]=="icon/item/test_blade.png")==next(r for r in old_index["files"] if r["path"]=="icon/item/test_blade.png")
    validate_public_assets(public)


def test_compound_paths_recover_bounded_stale_lengths_and_reject_unsafe_paths():
    path="res/mob/test/fixture.bsr"
    payload=bytearray(b"JMXVCPD 0101"+bytes(28)+struct.pack("<I",1)+_string(path))
    struct.pack_into("<I",payload,16,40)
    struct.pack_into("<I",payload,44,len(path)+2)
    paths,corrections=models.compound_resource(bytes(payload))
    assert paths==[path] and corrections==[{"path":path,"delta":-2}]
    struct.pack_into("<I",payload,44,len(path)+17)
    with pytest.raises(models.ModelError,match="excessively"):
        models.compound_resource(bytes(payload))


def test_compound_character_exports_one_named_picture_and_audits_parts(tmp_path,monkeypatch):
    resources=_authored_resources()
    payload=bytearray(b"JMXVCPD 0101"+bytes(28)+struct.pack("<I",1)+_string("res/mob/test/fixture.bsr"))
    struct.pack_into("<I",payload,16,40)
    resources["res/char/test/assembled.cpd"]=bytes(payload)
    source=_monster_source(tmp_path,monkeypatch,resources)
    fields=["0"]*55
    fields[0],fields[1],fields[2],fields[15],fields[52]="1","42","MOB_ASSEMBLED","8","char/test/assembled.cpd"
    _FakeArchive.payloads["server_dep/silkroad/textdata/characterdata_5000.txt"]=_text("\t".join(fields))
    result=exporter.export_dataset(source,tmp_path/"exports","fixture-key",tmp_path/"game-assets",unique_monsters=True)
    assert result["monsterRenderCount"]==1
    assert (tmp_path/"game-assets/monsters/assembled.png").is_file()
    audit=json.loads((Path(result["auditPath"])/"tables/monster-renders.json").read_text())
    assert len(audit[0]["parts"])==1
    validate_public_assets(tmp_path/"game-assets")


def test_mesh_uses_actual_skin_boundary_when_custom_face_header_is_stale():
    original=_authored_resources()["prim/mesh/fixture.bms"]
    changed=bytearray(original)
    face_offset=struct.unpack_from("<I",changed,20)[0]
    struct.pack_into("<I",changed,20,face_offset-9)
    parsed=models.mesh(bytes(changed))
    assert parsed["indices"]==[0,1,2]
    assert parsed["faceOffsetDelta"]==9


def test_same_model_basename_uses_resource_folders_instead_of_numeric_ids(tmp_path,monkeypatch):
    resources=_authored_resources()
    resources["res/test/fixture.bsr"]=resources["res/mob/test/fixture.bsr"]
    resources["res/deeper/test/fixture.bsr"]=resources["res/mob/test/fixture.bsr"]
    source=_monster_source(tmp_path,monkeypatch,resources)
    fields=["0"]*55
    fields[0],fields[1],fields[2],fields[15],fields[52]="1","42","MOB_TEST","3","res/test/fixture.bsr, res/deeper/test/fixture.bsr"
    _FakeArchive.payloads["server_dep/silkroad/textdata/characterdata_5000.txt"]=_text("\t".join(fields))
    result=exporter.export_dataset(source,tmp_path/"exports","fixture-key",tmp_path/"game-assets",unique_monsters=True)
    assert result["monsterRenderCount"]==2
    files={p.name for p in (tmp_path/"game-assets/monsters").iterdir()}
    assert files=={"test_fixture.png","deeper_test_fixture.png"}
