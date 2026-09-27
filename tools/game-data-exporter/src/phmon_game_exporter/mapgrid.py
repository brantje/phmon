"""Infer minimap tile-grid orientation from adjacent raster edges."""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
from statistics import fmean
from typing import Any

from PIL import Image, ImageChops, ImageStat


EDGE_BAND_PIXELS = 8
MIN_ADJACENT_PAIRS = 8
MIN_PREFERRED_SHARE = 0.8
MAX_PREFERRED_MEAN_RATIO = 0.75


def _mean_rgb_error(first: Image.Image, second: Image.Image) -> float:
    if first.size != second.size:
        raise ValueError("neighboring minimap edge bands have different dimensions")
    return fmean(ImageStat.Stat(ImageChops.difference(first, second)).mean[:3])


def infer_tile_grid_orientation(
    records: list[dict[str, Any]],
    asset_paths_by_key: dict[str, Path],
) -> dict[str, Any]:
    """Return only edge-continuity-supported X/Y index directions for one tile set."""
    coordinates: dict[tuple[int, int], str] = {}
    for record in records:
        coordinate = (record["x"], record["y"])
        if coordinate in coordinates:
            raise ValueError(f"duplicate minimap tile coordinate: {coordinate}")
        key = record["assetKey"]
        if key not in asset_paths_by_key:
            raise ValueError(f"minimap tile asset is missing: {key}")
        coordinates[coordinate] = key

    @lru_cache(maxsize=256)
    def load_image(key: str) -> Image.Image:
        with Image.open(asset_paths_by_key[key]) as image:
            return image.convert("RGB")

    horizontal: list[tuple[float, float]] = []
    vertical: list[tuple[float, float]] = []
    skipped_horizontal_pairs = 0
    skipped_vertical_pairs = 0

    for (x, y), key in sorted(coordinates.items()):
        image = load_image(key)

        right_key = coordinates.get((x + 1, y))
        if right_key is not None:
            right = load_image(right_key)
            if image.size != right.size or image.width < EDGE_BAND_PIXELS or image.height < EDGE_BAND_PIXELS:
                skipped_horizontal_pairs += 1
            else:
                left_edge = image.crop((0, 0, EDGE_BAND_PIXELS, image.height))
                right_edge = right.crop((0, 0, EDGE_BAND_PIXELS, right.height))
                image_right = image.crop((image.width - EDGE_BAND_PIXELS, 0, image.width, image.height))
                right_right = right.crop((right.width - EDGE_BAND_PIXELS, 0, right.width, right.height))
                horizontal.append((
                    _mean_rgb_error(image_right.transpose(Image.Transpose.FLIP_LEFT_RIGHT), right_edge),
                    _mean_rgb_error(left_edge, right_right.transpose(Image.Transpose.FLIP_LEFT_RIGHT)),
                ))

        above_key = coordinates.get((x, y + 1))
        if above_key is not None:
            above = load_image(above_key)
            if image.size != above.size or image.width < EDGE_BAND_PIXELS or image.height < EDGE_BAND_PIXELS:
                skipped_vertical_pairs += 1
            else:
                top_edge = image.crop((0, 0, image.width, EDGE_BAND_PIXELS))
                bottom_edge = image.crop((0, image.height - EDGE_BAND_PIXELS, image.width, image.height))
                above_top = above.crop((0, 0, above.width, EDGE_BAND_PIXELS))
                above_bottom = above.crop((0, above.height - EDGE_BAND_PIXELS, above.width, above.height))
                vertical.append((
                    _mean_rgb_error(top_edge, above_bottom.transpose(Image.Transpose.FLIP_TOP_BOTTOM)),
                    _mean_rgb_error(bottom_edge.transpose(Image.Transpose.FLIP_TOP_BOTTOM), above_top),
                ))

    def summarize(pairs: list[tuple[float, float]], positive_name: str, negative_name: str) -> dict[str, Any]:
        positive_wins = sum(positive < negative for positive, negative in pairs)
        negative_wins = sum(negative < positive for positive, negative in pairs)
        ties = len(pairs) - positive_wins - negative_wins
        positive_mean = fmean(positive for positive, _ in pairs) if pairs else None
        negative_mean = fmean(negative for _, negative in pairs) if pairs else None
        direction = None
        if len(pairs) >= MIN_ADJACENT_PAIRS and positive_mean is not None and negative_mean is not None:
            if positive_wins / len(pairs) >= MIN_PREFERRED_SHARE and positive_mean <= negative_mean * MAX_PREFERRED_MEAN_RATIO:
                direction = positive_name
            elif negative_wins / len(pairs) >= MIN_PREFERRED_SHARE and negative_mean <= positive_mean * MAX_PREFERRED_MEAN_RATIO:
                direction = negative_name
        return {
            "adjacentPairCount": len(pairs),
            "skippedAdjacentPairCount": skipped_horizontal_pairs if positive_name in ("right", "left") else skipped_vertical_pairs,
            f"meanRgbErrorIncreasing{positive_name.title().replace('-', '')}": round(positive_mean, 3) if positive_mean is not None else None,
            f"meanRgbErrorIncreasing{negative_name.title().replace('-', '')}": round(negative_mean, 3) if negative_mean is not None else None,
            f"increasing{positive_name.title().replace('-', '')}LowerErrorPairs": positive_wins,
            f"increasing{negative_name.title().replace('-', '')}LowerErrorPairs": negative_wins,
            "equalErrorPairs": ties,
            "increasingDirection": direction,
        }

    horizontal_summary = summarize(horizontal, "right", "left")
    vertical_summary = summarize(vertical, "up", "down")
    supported_axes = sum((horizontal_summary["increasingDirection"] is not None, vertical_summary["increasingDirection"] is not None))
    if supported_axes == 2:
        status = "edge-continuity-supported"
    elif supported_axes == 1:
        status = "partial-edge-continuity-support"
    elif len(horizontal) < MIN_ADJACENT_PAIRS or len(vertical) < MIN_ADJACENT_PAIRS:
        status = "insufficient-adjacencies"
    else:
        status = "inconclusive"

    return {
        "status": status,
        "tileCount": len(records),
        "xIncreasingDirection": horizontal_summary["increasingDirection"],
        "yIncreasingDirection": vertical_summary["increasingDirection"],
        "method": "8-pixel adjacent edge-band mean absolute RGB error; compare both neighbor orders with edge-distance alignment",
        "classificationCriteria": {
            "minimumAdjacentPairsPerAxis": MIN_ADJACENT_PAIRS,
                "minimumPreferredShareOfAllPairs": MIN_PREFERRED_SHARE,
            "maximumPreferredToAlternateMeanErrorRatio": MAX_PREFERRED_MEAN_RATIO,
        },
        "horizontalEvidence": horizontal_summary,
        "verticalEvidence": vertical_summary,
    }
