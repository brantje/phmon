from __future__ import annotations

from pathlib import Path

from PIL import Image

from phmon_game_exporter.mapgrid import infer_tile_grid_orientation


def test_edge_continuity_supports_x_right_and_y_up_for_synthetic_grid(tmp_path):
    size = 32
    records = []
    asset_paths = {}
    for x in range(4):
        for y in range(4):
            pixels = []
            for row in range(size):
                for column in range(size):
                    world_x = x * size + column
                    world_y = y * size + (size - 1 - row)
                    pixels.append((world_x, world_y, world_x + world_y))
            image = Image.new("RGB", (size, size))
            image.putdata(pixels)
            key = f"tile:{x}:{y}"
            path = tmp_path / f"{x}-{y}.png"
            image.save(path)
            asset_paths[key] = path
            records.append({"x": x, "y": y, "assetKey": key})

    orientation = infer_tile_grid_orientation(records, asset_paths)

    assert orientation["status"] == "edge-continuity-supported"
    assert orientation["xIncreasingDirection"] == "right"
    assert orientation["yIncreasingDirection"] == "up"
    assert orientation["horizontalEvidence"]["adjacentPairCount"] == 12
    assert orientation["verticalEvidence"]["adjacentPairCount"] == 12
