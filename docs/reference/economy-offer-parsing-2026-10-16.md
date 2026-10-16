# Global offer tags (heuristic text parsing)

Operator-authorized static inspection of `phMonitor-v0.5.0.exe` (SHA-256
`6768f32f4615e950...`) found that phMonitor parses offers server-side from chat text
(`main.parseMarketOffer`, `splitMarketSegments`, `marketItemType`, `marketSubtype`,
`marketSeal`, `marketRace`, `marketDegree`, `marketPrice`, `marketNotes`,
`marketIconPath`). The behaviour and keyword families were recorded and PhMon's parser
in `server/internal/chat/offer_parse.go` was written independently; no code or assets
were copied.

PhMon behaviour: a message is split into fragments (`|`, newline, `;`, `,`, ` / `,
` or `, ` and `); a fragment without an item kind inherits it from the previous one when
it only adds a seal, degree or plus. Tags (type, subcategory, part, degree 1-14, seal,
race, plus, price, currency, notes) are stored in `global_offer_items` (migration 36)
by a lazy, bounded parse inside `Offers`, versioned by `OfferParserVersion`. Icons are
resolved at read time from the server's game-data catalog by race/subtype/degree.
Tags are text heuristics: players abbreviate freely, so they can be wrong or missing,
and they are not item identity, completed trades or prices.
