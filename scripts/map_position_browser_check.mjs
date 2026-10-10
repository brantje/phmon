#!/usr/bin/env node

// Browser-only replay of captured live contracts. No commands or fixture writes
// reach the backend. Requires Playwright (PHMON_PLAYWRIGHT_MODULE can point to it).
import assert from "node:assert/strict";

const { chromium } = await import(
  process.env.PHMON_PLAYWRIGHT_MODULE || "playwright"
);
const origin = process.env.SMOKE_WEB_URL || "http://127.0.0.1:3005";
const characterID = process.env.MAP_CHARACTER_ID;
assert(characterID, "MAP_CHARACTER_ID is required");
assert(
  process.env.OPERATOR_ACCESS_SECRET,
  "OPERATOR_ACCESS_SECRET is required",
);
const auth = await fetch(`${origin}/api/auth/login`, {
  method: "POST",
  headers: { origin, "content-type": "application/json" },
  body: JSON.stringify({ secret: process.env.OPERATOR_ACCESS_SECRET }),
});
assert.equal(auth.status, 200, "operator authentication failed");
const cookie = auth.headers.get("set-cookie").split(";")[0];
const profileResponse = await fetch(
  `${origin}/api/map/profile?server=${encodeURIComponent(process.env.MAP_SERVER || "Greatest")}`,
  { headers: { cookie } },
);
assert.equal(profileResponse.status, 200, "map profile unavailable");
const profile = await profileResponse.json();
assert.equal(profile.coordinate_transform_status, "outdoor-region-grid");
const outdoorRegion = (x, y) =>
  Math.floor(x / 192) + 135 + (Math.ceil(y / 192) + 91) * 256;
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox"],
});
try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  await page.context().addCookies([
    {
      name: cookie.split("=")[0],
      value: cookie.slice(cookie.indexOf("=") + 1),
      url: origin,
    },
  ]);
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  // Guard against any bot command or other application mutation during the check.
  await page.route("**/api/**", (route) => {
    if (route.request().method() === "GET") return route.continue();
    errors.push(
      `unexpected mutation: ${route.request().method()} ${route.request().url()}`,
    );
    return route.abort();
  });
  await page.addInitScript(() => {
    window.popupBuilds = 0;
    window.crossingDraws = 0;
    const stroke = CanvasRenderingContext2D.prototype.stroke;
    CanvasRenderingContext2D.prototype.stroke = function (...args) {
      if (
        this.strokeStyle === "#f4d27a" &&
        this.getLineDash().join(",") === "2,6"
      ) {
        window.crossingDraws++;
      }
      return stroke.apply(this, args);
    };
    const descriptor = Object.getOwnPropertyDescriptor(
      Element.prototype,
      "className",
    );
    Object.defineProperty(Element.prototype, "className", {
      ...descriptor,
      set(value) {
        if (typeof value === "string" && value.startsWith("phmon-map-detail "))
          window.popupBuilds++;
        descriptor.set.call(this, value);
      },
    });
  });
  let frozen = false;
  let socket;
  let mapFrame;
  let positionFrame;
  let latestPosition;
  await page.routeWebSocket("**/api/live", (ws) => {
    socket = ws;
    const server = ws.connectToServer();
    ws.onMessage((message) => server.send(message));
    server.onMessage((message) => {
      const frame = JSON.parse(String(message));
      if (frame.stream === "map" && frame.data?.characters) mapFrame = frame;
      if (frame.stream === "positions") {
        const position = frame.data?.positions?.find(
          (p) => p.character_id === characterID,
        );
        if (position) {
          latestPosition = position;
          positionFrame = frame;
        }
      }
      if (!frozen) ws.send(message);
    });
  });
  await page.goto(
    `${origin}/map?server=${encodeURIComponent(process.env.MAP_SERVER || "Greatest")}&area=world&floor=world&character_id=${characterID}`,
    { waitUntil: "domcontentloaded" },
  );
  await page.waitForFunction(() =>
    document.querySelector(".phmon-map-marker--character"),
  );
  for (let attempts = 0; !latestPosition || !mapFrame; attempts++) {
    assert(attempts < 50, "live position/map baseline unavailable");
    await page.waitForTimeout(100);
  }
  frozen = true;
  const capturedMap = structuredClone(mapFrame);
  const capturedPosition = structuredClone(latestPosition);
  const character = capturedMap.data.characters.find(
    (c) => c.character_id === characterID,
  );
  assert(character, "target character missing from map snapshot");
  const marker = page
    .getByTitle(character.name, { exact: true })
    .and(page.locator(".phmon-map-marker--character"));
  await marker.waitFor();
  await page.waitForTimeout(300);
  await page.evaluate(() => {
    window.popupBuilds = 0;
  });
  const initial = await marker.evaluate((element) => element.style.transform);
  let sequence = capturedPosition.sequence + 1000;
  const sendPosition = (x) =>
    socket.send(
      JSON.stringify({
        ...positionFrame,
        type: "delta",
        data: {
          positions: [
            {
              ...capturedPosition,
              sequence: ++sequence,
              x,
              region: outdoorRegion(x, capturedPosition.y),
              observed_at: new Date().toISOString(),
            },
          ],
          removed: [],
        },
      }),
    );
  // Move in the browser fixture, then deliver identical targets fast enough to
  // interrupt the marker animation if unchanged refreshes restart it.
  const targetX = capturedPosition.x + 100;
  sendPosition(targetX);
  for (let i = 0; i < 15; i++) {
    await page.waitForTimeout(25);
    socket.send(JSON.stringify(capturedMap));
    sendPosition(targetX);
  }
  const settled = await marker.evaluate((element) => element.style.transform);
  assert.notEqual(settled, initial, "position delta did not move the marker");
  await page.waitForTimeout(250);
  assert.equal(
    await marker.evaluate((element) => element.style.transform),
    settled,
    "unchanged refreshes restarted or rewound marker animation",
  );
  assert.equal(
    await page.evaluate(() => window.popupBuilds),
    0,
    "closed popup DOM was built during position refreshes",
  );

  // Recenter after capture: a real teleport can occur between the map snapshot
  // and the newest position frame used for these browser-only fixtures.
  await marker.dispatchEvent("click");
  await page
    .getByRole("button", {
      name: `Follow ${character.name} on the map`,
      exact: true,
    })
    .click();
  await page.waitForTimeout(500);
  await page
    .getByRole("button", {
      name: `Stop following ${character.name}`,
      exact: true,
    })
    .click();

  // A teleport is separate presentation geometry, never a walking segment.
  // Route fixtures stay inside this browser and also work between real trips.
  const route = {
    command_id: "browser-fixture-route",
    character_id: characterID,
    session_id: character.session_id,
    route_sequence: 2147483647,
    server: character.server,
    dataset_id: profile.dataset_id,
    dataset_version: profile.dataset_version,
    area_id: "world",
    floor_id: "world",
    destination: {
      region: outdoorRegion(capturedPosition.x + 5, capturedPosition.y),
      x: capturedPosition.x + 5,
      y: capturedPosition.y,
      z: capturedPosition.z || 0,
    },
    status: "moving",
    updated_at: new Date().toISOString(),
    blocks: [],
  };
  const crossing = {
    from_area_id: "world",
    from_floor_id: "world",
    from: {
      region: outdoorRegion(capturedPosition.x, capturedPosition.y),
      x: capturedPosition.x,
      y: capturedPosition.y,
      z: capturedPosition.z || 0,
    },
    to_area_id: "world",
    to_floor_id: "world",
    to: {
      region: outdoorRegion(capturedPosition.x + 5, capturedPosition.y),
      x: capturedPosition.x + 5,
      y: capturedPosition.y,
      z: capturedPosition.z || 0,
    },
  };
  const crossingFrame = structuredClone(capturedMap);
  crossingFrame.data.navigation = [
    { ...route, teleport_crossings: [crossing] },
  ];
  await page.evaluate(() => {
    window.crossingDraws = 0;
  });
  socket.send(JSON.stringify(crossingFrame));
  await page.waitForFunction(() => window.crossingDraws > 0);
  crossingFrame.data.navigation[0].teleport_crossings = [];
  await page.waitForTimeout(100);
  await page.evaluate(() => {
    window.crossingDraws = 0;
  });
  socket.send(JSON.stringify(crossingFrame));
  await page.waitForTimeout(250);
  assert.equal(
    await page.evaluate(() => window.crossingDraws),
    0,
    "completed teleport crossing was still drawn",
  );
  socket.send(JSON.stringify(capturedMap));

  const hoverFrame = structuredClone(capturedMap);
  const from = {
    ...crossing.from,
    x: capturedPosition.x + 40,
    y: capturedPosition.y + 60,
    region: outdoorRegion(capturedPosition.x + 40, capturedPosition.y + 60),
  };
  const to = {
    ...from,
    x: capturedPosition.x + 160,
    region: outdoorRegion(capturedPosition.x + 160, capturedPosition.y + 60),
  };
  const assertHoverSurvives = async (kind) => {
    const teleport = kind === "crossing";
    hoverFrame.data.navigation = [
      {
        ...route,
        blocks: teleport
          ? []
          : [{ area_id: "world", floor_id: "world", points: [from, to] }],
        teleport_crossings: teleport ? [{ ...crossing, from, to }] : [],
      },
    ];
    await page.mouse.move(20, 20);
    socket.send(JSON.stringify(hoverFrame));
    await page.waitForTimeout(350);
    const color = teleport ? [244, 210, 122] : [55, 214, 209];
    // Read the rendered canvas pixels so pointer coordinates remain correct
    // through Leaflet's transforms, panning and navigation-panel resizing.
    const target = await page.evaluate((color) => {
      let best;
      for (const canvas of document.querySelectorAll(
        ".leaflet-overlay-pane canvas",
      )) {
        const rect = canvas.getBoundingClientRect();
        const pixels = canvas
          .getContext("2d")
          .getImageData(0, 0, canvas.width, canvas.height).data;
        for (let y = 0; y < canvas.height; y++) {
          const screenY = rect.top + ((y + 0.5) * rect.height) / canvas.height;
          if (screenY < 100 || screenY > innerHeight - 100) continue;
          const row = [];
          for (let x = 0; x < canvas.width; x++) {
            const offset = (y * canvas.width + x) * 4;
            if (
              Math.abs(pixels[offset] - color[0]) <= 2 &&
              Math.abs(pixels[offset + 1] - color[1]) <= 2 &&
              Math.abs(pixels[offset + 2] - color[2]) <= 2 &&
              pixels[offset + 3] >= 30
            ) {
              const screenX =
                rect.left + ((x + 0.5) * rect.width) / canvas.width;
              if (screenX > 300 && screenX < 1050) row.push(screenX);
            }
          }
          if (row.length > (best?.count || 0))
            best = {
              count: row.length,
              x: row[Math.floor(row.length / 2)],
              y: screenY,
            };
        }
      }
      return best;
    }, color);
    if (!(target?.count > 10)) {
      console.log({ kind, target });
      await page.screenshot({ path: `/tmp/phmon-route-hover-${kind}.png` });
    }
    assert(target?.count > 10, `${kind} fixture line was not rendered`);
    // Leaflet throttles canvas hit-testing after each pointer movement.
    await page.waitForTimeout(100);
    await page.mouse.move(Math.round(target.x), Math.round(target.y));
    const tooltip = page.locator(".leaflet-tooltip").filter({
      hasText: `${character.name} · ${teleport ? "teleport / ferry crossing" : "remaining route"}`,
    });
    try {
      await tooltip.waitFor({ state: "visible", timeout: 5000 });
    } catch (error) {
      console.log({
        kind,
        target,
        tooltips: await page.locator(".leaflet-tooltip").allTextContents(),
        map: await page.locator(".map-canvas").boundingBox(),
      });
      await page.screenshot({ path: `/tmp/phmon-route-hover-${kind}.png` });
      throw error;
    }
    const original = await tooltip.elementHandle();
    for (let i = 0; i < 15; i++) {
      socket.send(JSON.stringify(hoverFrame));
      sendPosition(targetX + i);
      await page.waitForTimeout(100);
      assert(
        await original.evaluate(
          (element) =>
            element.isConnected && getComputedStyle(element).opacity !== "0",
        ),
        `${kind} tooltip disappeared during live refresh`,
      );
    }
    await page.mouse.move(20, 20);
    await tooltip.waitFor({ state: "hidden" });
    hoverFrame.data.navigation = [];
    socket.send(JSON.stringify(hoverFrame));
    await page.waitForTimeout(100);
  };
  await assertHoverSurvives("walk");
  await assertHoverSurvives("crossing");
  socket.send(JSON.stringify(capturedMap));

  // A hidden popup must use current data when opened, and remain current while open.
  const renderedPlayerNames = await page
    .locator(".phmon-map-marker--player")
    .evaluateAll((elements) => elements.map((element) => element.title));
  const player = capturedMap.data.players?.players?.find((p) =>
    renderedPlayerNames.includes(p.name),
  );
  assert(player, "other-player fixture required for popup freshness check");
  const pin = page
    .getByTitle(player.name, { exact: true })
    .and(page.locator(".phmon-map-marker--player"));
  const replayPlayer = (guild) => {
    const frame = structuredClone(capturedMap);
    frame.data.players.players.find((p) => p.name === player.name).guild =
      guild;
    socket.send(JSON.stringify(frame));
  };
  replayPlayer("Popup fixture one");
  await page.waitForTimeout(100);
  await pin.dispatchEvent("click");
  await page.getByText("Popup fixture one", { exact: true }).waitFor();
  replayPlayer("Popup fixture two");
  await page.getByText("Popup fixture two", { exact: true }).waitFor();
  await page.locator(".map-canvas").click({ position: { x: 20, y: 20 } });
  replayPlayer("Popup fixture three");
  await page.waitForTimeout(100);
  await pin.dispatchEvent("click");
  await page.getByText("Popup fixture three", { exact: true }).waitFor();
  await marker.dispatchEvent("click");
  await page
    .getByRole("button", {
      name: `Follow ${character.name} on the map`,
      exact: true,
    })
    .click();
  await page.waitForTimeout(500);
  sendPosition(targetX + 24);
  await page.waitForTimeout(500);
  const centered = await marker.evaluate((element) => {
    const pin = element.getBoundingClientRect();
    const map = document.querySelector(".map-canvas").getBoundingClientRect();
    return Math.hypot(
      pin.x + pin.width / 2 - map.x - map.width / 2,
      pin.y + pin.height / 2 - map.y - map.height / 2,
    );
  });
  assert(centered < 3, "follow did not keep the moving character centered");
  await page
    .getByRole("button", {
      name: `Stop following ${character.name}`,
      exact: true,
    })
    .click();
  for (const [width, height] of [
    [1280, 800],
    [390, 844],
  ]) {
    await page.setViewportSize({ width, height });
    await page.waitForTimeout(250);
    assert(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
      `map overflowed at ${width} × ${height}`,
    );
  }
  assert.deepEqual(errors, []);
  console.log(
    "Map browser replay passed: movement survives unchanged refreshes, walking/crossing hover survives live updates, teleport crossings draw and clear, zero hidden popup builds, closed/open/reopened popup data stays current, selection/follow and responsive widths work; no backend mutations.",
  );
} finally {
  await browser.close();
}
