"""Verified phMonitor v0.5.0 player-model to character portrait mapping."""

from __future__ import annotations

PORTRAIT_MODEL_RANGES = (
    ("ch", "man", 1907, 1919),
    ("ch", "woman", 1920, 1932),
    ("eu", "man", 14717, 14729),
    ("eu", "man", 14875, 14887),
    ("eu", "woman", 14730, 14742),
    ("eu", "woman", 14888, 14900),
)


def portrait_for_model(model: int) -> tuple[str, str, int, str] | None:
    """Return race, gender, portrait ordinal and filename for known models."""
    if isinstance(model, bool) or not isinstance(model, int) or model <= 0:
        return None
    for race, gender, first, last in PORTRAIT_MODEL_RANGES:
        if first <= model <= last:
            ordinal = model - first + 1
            return race, gender, ordinal, f"char_{race}_{gender}{ordinal}.png"
    return None


def portrait_source_path(model: int) -> str | None:
    """Return the verified Media.pk2 DDJ path for a known player model."""
    portrait = portrait_for_model(model)
    if portrait is None:
        return None
    filename = portrait[3].rsplit(".", 1)[0] + ".ddj"
    return f"interface/character/{filename}"
