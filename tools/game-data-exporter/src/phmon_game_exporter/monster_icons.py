"""Observed monster textures and PhMon's existing numeric type mapping."""

# Party variants share one badge. Rank artwork stays a separate asset.
MONSTER_ICONS = (
    (0, "general", "interface/targetwindow/tw_icon_normal.ddj", "rank", None),
    (1, "champion", "interface/targetwindow/tw_icon_champion.ddj", "rank", None),
    (4, "giant", "interface/targetwindow/tw_icon_giant.ddj", "rank", None),
    (16, "party_general", "icon/etc/europe_partymob.ddj", "party_badge", 0),
    (17, "party_champion", "icon/etc/europe_partymob.ddj", "party_badge", 1),
    (20, "party_giant", "icon/etc/europe_partymob.ddj", "party_badge", 4),
)
