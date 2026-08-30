(function () {
  const featureFlags = {
    uprd: false,
    map: false,
  };

  const state = {
    page: "terminal",
    ports: [],
    positions: new Map(),
    rings: new Map(),
    colors: new Map(),
    map: { x: 0, y: 0, scale: 1, drag: false, startX: 0, startY: 0, originX: 0, originY: 0 },
  };

  const $ = (id) => document.getElementById(id);
  const helpIcon = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M11 18h2v-2h-2v2m1-16A10 10 0 1 0 12 22 10 10 0 0 0 12 2m0 18a8 8 0 1 1 0-16 8 8 0 0 1 0 16m0-14a4 4 0 0 0-4 4h2a2 2 0 1 1 2.35 1.97C10.5 12.28 11 14 11 14h2c0-1.5 3-1.75 3-4a4 4 0 0 0-4-4Z"/></svg>';
  const applyDynamicLanguage = () => {
    if (typeof applyUILanguage === "function") applyUILanguage(uiLanguage);
    const experimental = $("uprdirectExperimental");
    if (experimental?.parentElement?.lastChild) experimental.parentElement.lastChild.nodeValue = uiLanguage === "en" ? " UPRD - Map" : " UPRD - Mapa";
    const uprdTab = $("configUPRDirectTab");
    if (uprdTab) uprdTab.textContent = "UPRdirect";
    const beaconTab = $("configBeaconTab");
    if (beaconTab) beaconTab.textContent = uiLanguage === "en" ? "Beacon" : "Beacon";
    const appearanceTab = $("configAppearanceTab");
    if (appearanceTab) appearanceTab.textContent = uiLanguage === "en" ? "Appearance & sounds" : "Wygląd i dźwięki";
    const cardDescription = $("uprdCardDescription");
    if (cardDescription) cardDescription.textContent = uiLanguage === "en"
      ? "UPRD sends a short report of local RF knowledge over standard AX.25 UI. It is independent of the classic beacon."
      : "UPRD wysyła krótki raport lokalnej wiedzy RF nad standardowym AX.25 UI. Funkcja jest niezależna od klasycznego beaconu.";
    const enabledLabel = $("uprdEnabledLabel");
    if (enabledLabel) enabledLabel.textContent = uiLanguage === "en" ? "Enable UPRdirect features" : "Włącz funkcje UPRdirect";
    const intervalLabel = $("uprdIntervalLabel");
    if (intervalLabel) intervalLabel.textContent = uiLanguage === "en" ? "Interval (minutes)" : "Interwał (minuty)";
    const limitLabel = $("uprdLimitLabel");
    if (limitLabel) limitLabel.textContent = uiLanguage === "en" ? "MHEARD limit (max.)" : "Limit MHEARD (maks.)";
    const helpButton = $("uprdHelpButton");
    if (helpButton) helpButton.setAttribute("aria-label", uiLanguage === "en" ? "UPRD help" : "Pomoc UPRD");
    const info = $("uprdInfoText");
    if (info) info.textContent = uiLanguage === "en"
      ? "Beacon and UPRD are independent and can be enabled at the same time."
      : "Beacon i UPRD są niezależne i mogą działać jednocześnie.";
    document.querySelectorAll("[data-mheard-help]").forEach((button) => {
      button.setAttribute("aria-label", uiLanguage === "en" ? "MHEARD help" : "Pomoc MHEARD");
    });
    if ($("mheardHelpText")) $("mheardHelpText").textContent = uiLanguage === "en"
      ? "MHEARD shows all stations heard locally and learned from UPRD status reports. Double-click a row to start a connection."
      : "MHEARD pokazuje wszystkie stacje usłyszane lokalnie oraz poznane ze statusów UPRD. Dwuklik w wiersz rozpoczyna połączenie.";
    if ($("mheardHelpUprdLabel")) $("mheardHelpUprdLabel").textContent = uiLanguage === "en" ? "green frame — station transmitting UPRD status" : "zielona ramka — stacja nadająca status UPRD";
    if ($("mheardHelpReportedLabel")) $("mheardHelpReportedLabel").textContent = uiLanguage === "en" ? "blue frame — station heard and listed in another station's UPRD status" : "niebieska ramka — stacja usłyszana i podana w statusie UPRD innej stacji";
  };
  const esc = (v) => String(v ?? "").replace(/[&<>"]/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
  }[c]));

  const color = (port) => {
    if (!state.colors.has(port)) {
      let n = 0;
      for (const c of port) n = (n * 31 + c.charCodeAt(0)) % 360;
      state.colors.set(port, `hsl(${n},68%,58%)`);
    }
    return state.colors.get(port);
  };

  const freshness = (t) => (typeof activityState === "function"
    ? activityState(t)
    : (() => {
        const age = (Date.now() - new Date(t).getTime()) / 60000;
        return age < 15 ? "green" : age < 45 ? "yellow" : "red";
      })());

  function addNav() {
    const nav = document.querySelector("nav");
    if (!nav) return;

    const existing = $("navMap");
    if (existing) {
      existing.hidden = !featureFlags.map;
      existing.disabled = !featureFlags.map;
      existing.onclick = featureFlags.map ? () => open("map") : null;
      return;
    }

    if (!featureFlags.map) return;

    const spacer = nav.querySelector(".nav-spacer");
    const button = document.createElement("button");
    button.id = "navMap";
    button.textContent = "Mapa";
    button.onclick = () => open("map");
    nav.insertBefore(button, spacer);
  }

  function addConfig() {
    const panel = $("uprDirectConfigPanel");
    if (!panel || $("uprdConfigCard")) return;

    const beacon = $("beaconConfigCard");
    const beaconPanel = $("beaconConfigPanel");
    if (beacon && beaconPanel) beaconPanel.appendChild(beacon);

    const card = document.createElement("div");
    card.id = "uprdConfigCard";
    card.className = "settings-card";
    card.innerHTML = `
      <div class="settings-card-heading"><h3 id="uprdCardTitle">UPRdirect</h3><button id="uprdHelpButton" class="help-button" type="button" aria-label="Pomoc UPRD">${helpIcon}</button></div>
      <p id="uprdCardDescription">UPRD wysyła krótki raport lokalnej wiedzy RF nad standardowym AX.25 UI. Funkcja jest niezależna od klasycznego beaconu.</p>
      <label class="check uprd-master-check"><input id="uprdEnabled" type="checkbox"><span id="uprdEnabledLabel">Włącz funkcje UPRdirect</span></label>
      <div id="uprdSettingsFields" class="field-grid">
        <label><span id="uprdIntervalLabel">Interwał (minuty)</span><input id="uprdInterval" type="number" min="1" max="1440"></label>
        <label><span id="uprdLimitLabel">Limit MHEARD (maks.)</span><input id="uprdLimit" type="number" min="1" max="10"></label>
      </div>
      <div id="uprdHelpModal" class="uprd-help-modal" hidden role="dialog" aria-modal="true" aria-labelledby="uprdHelpTitle">
        <div class="uprd-help-content"><div class="settings-card-heading"><h3 id="uprdHelpTitle">UPRD</h3><button id="uprdHelpClose" class="help-button" type="button" aria-label="Zamknij">×</button></div><p id="uprdHelpText"></p><button id="uprdHelpOk" class="secondary" type="button">OK</button></div>
      </div>
    `;

    card.hidden = false;
    panel.appendChild(card);
    $("configUPRDirectTab")?.toggleAttribute("hidden", false);

    const oldFill = window.fillApplication;
    if (typeof oldFill === "function") {
      window.fillApplication = (c) => {
        oldFill(c);
        const u = c.UPRD || {};
        $("uprdEnabled").checked = !!u.Enabled;
        $("uprdInterval").value = Math.max(1, Math.round((u.IntervalSeconds || 600) / 60));
        $("uprdLimit").value = u.MHeardLimit || 5;
        applyExperimentalVisibility();
      };
    }

    $("uprdEnabled").onchange = () => { applyExperimentalVisibility(); };
    const helpText = () => $("uprdHelpText").textContent = uiLanguage === "en"
      ? "UPRD is a direct AX.25 status report for sharing the station locator and recently heard stations. It also reports whether the operator is present. The report is sent periodically and when the web panel opens or the last panel closes. UPRD and the classic beacon are independent."
      : "UPRD to bezpośredni raport AX.25 zawierający lokator stacji, ostatnio słyszane stacje oraz informację o obecności operatora. Raport jest wysyłany okresowo, a także przy otwarciu panelu WWW i po zamknięciu ostatniego panelu. UPRD i klasyczny beacon działają niezależnie.";
    const showHelp = () => { helpText(); $("uprdHelpModal").hidden = false; };
    const hideHelp = () => { $("uprdHelpModal").hidden = true; };
    $("uprdHelpButton").onclick = showHelp;
    $("uprdHelpClose").onclick = hideHelp;
    $("uprdHelpOk").onclick = hideHelp;
    $("uprdHelpModal").onclick = (event) => { if (event.target === $("uprdHelpModal")) hideHelp(); };
    applyDynamicLanguage();

    const save = $("saveConfig")?.onclick;
    if (save) {
      $("saveConfig").onclick = async () => {
        if (typeof configModel !== "undefined" && configModel) {
          configModel.UPRD = {
            Enabled: $("uprdEnabled").checked,
            IntervalSeconds: (Number($("uprdInterval").value) || 10) * 60,
            MHeardLimit: Math.min(10, Math.max(1, Number($("uprdLimit").value) || 5)),
          };
        }
        return save();
      };
    }
  }

  function applyExperimentalVisibility() {
    const enabled = $("uprdEnabled")?.checked === true;
    featureFlags.uprd = enabled;
    const mapFeature = $("uprdirectExperimental");
    if (!enabled && mapFeature) mapFeature.checked = false;
    mapFeature?.parentElement?.toggleAttribute("hidden", !enabled);

    const mapButton = $("navMap");
    if (mapButton) {
      mapButton.hidden = !featureFlags.map;
      mapButton.disabled = !featureFlags.map;
      mapButton.onclick = featureFlags.map ? () => open("map") : null;
    }
    $("configUPRDirectTab")?.toggleAttribute("hidden", false);
    $("uprdConfigCard")?.toggleAttribute("hidden", false);
    $("uprdSettingsFields")?.toggleAttribute("hidden", !enabled);
  }

  function setupExperimentalControls() {
    const card = $("experimentalFeaturesCard");
    const master = $("experimentalFeaturesToggle");
    const list = $("experimentalFeaturesList");
    const uprd = $("uprdirectExperimental");
    if (!card || !master || !list || !uprd) return;

    card.hidden = false;
    if (master.dataset.bound) return;
    master.dataset.bound = "true";
    uprd.dataset.bound = "true";

    const apply = () => {
      featureFlags.map = master.checked && uprd.checked && $("uprdEnabled")?.checked === true;
      list.hidden = !master.checked;
      if (uprd.parentElement?.lastChild) uprd.parentElement.lastChild.nodeValue = uiLanguage === "en" ? " UPRD - Map" : " UPRD - Mapa";
      applyExperimentalVisibility();
    };

    master.onchange = async () => {
      if (!master.checked) {
        uprd.checked = false;
        apply();
        return;
      }
      const message = "Wchodzisz do opcji eksperymentalnych na własne ryzyko. Funkcje mogą nie działać, zostać zmienione albo ostatecznie nie zostać wprowadzone do projektu. Czy chcesz kontynuować?";
      const confirmed = typeof showAppConfirm === "function"
        ? await showAppConfirm(message)
        : window.confirm(message);
      if (!confirmed) {
        master.checked = false;
        return;
      }
      list.hidden = false;
      uprd.checked = true;
      apply();
    };
    uprd.onchange = apply;
    apply();
  }

  function hide() {
    for (const id of ["terminalView", "monitorView", "infoView", "configView", "mheardView", "mapView"]) {
      const el = $(id);
      if (!el) continue;
      el.setAttribute("hidden", "");
      el.style.display = "none";
    }
  }

  function open(page) {
    if (page === "map" && !featureFlags.map) return;
    hide();
    state.page = page;

    for (const id of ["navTerminal", "navConfig", "navMonitor", "navInfo", "navMHeard", "navMap"]) {
      $(id)?.classList.toggle("active", id === `nav${page === "mheard" ? "MHeard" : page[0].toUpperCase() + page.slice(1)}`);
    }

    const el = $(page === "mheard" ? "mheardView" : "mapView");
    if (!el) return;
    el.removeAttribute("hidden");
    el.style.display = "block";
    page === "mheard" ? refreshMHeard() : refreshMap();
  }

  function wrapPages() {
    if (typeof window.showPage !== "function") return;
    const old = window.showPage;
    window.showPage = (page) => {
      hide();
      state.page = page;
      old(page);
    };
  }

  function syncNavigation() {
    const pages = { navTerminal: "terminal", navConfig: "config", navMonitor: "monitor", navInfo: "info" };
    for (const [id, page] of Object.entries(pages)) {
      $(id)?.addEventListener("click", () => {
        $("navMap")?.classList.remove("active");
        state.page = page;
      });
    }
  }

  function renderList(list, target) {
    const el = $(target);
    if (!el) return;

    el.innerHTML = list.length
      ? list.slice(0, 200).map((e) => `
        <div class="heard-row${e.source_type === "reported" ? " uprd-reported" : (e.operator_present !== undefined ? " uprd-sender" : "")}" data-call="${esc(e.callsign)}" data-port="${esc(e.port)}" data-via="${esc(e.via || "")}">
          <strong>${e.operator_present !== undefined && typeof operatorPresenceIcon === "function" ? operatorPresenceIcon(e.operator_present) : ""}${esc(e.callsign)}</strong>
          <span class="mheard-via">${e.via ? "via" : esc(e.port)}</span>
          <time>${age(e.last_seen)}</time>
        </div>
      `).join("")
      : '<p class="empty-list">Brak odebranych stacji.</p>';
    applyDynamicLanguage();

    el.querySelectorAll(".heard-row").forEach((row) => {
      row.ondblclick = () => window.connectRadioStation?.(row.dataset.call, row.dataset.port, row.dataset.via || "");
    });
  }

  const age = (t) => {
    const n = new Date(t).getTime();
    return Number.isFinite(n) ? `${Math.max(0, Math.floor((Date.now() - n) / 60000))} min` : "—";
  };

  async function refreshMHeard() {
    try {
      const list = await fetch("/api/mheard", { cache: "no-store" }).then((r) => r.json());
      renderList(list, "mheardFullList");
    } catch {
      // no-op
    }
  }

  function setupPanelMHeard() {
    const head = document.querySelector(".mheard-panel .side-head");
    if (!head || head.querySelector("[data-mheard-help]")) return;

    window.discoveryMHeardOwner = true;
    const small = head.querySelector("small");
    if (small) small.remove();

    const help = document.createElement("button");
    help.className = "help-button mheard-help-button";
    help.type = "button";
    help.dataset.mheardHelp = "";
    help.setAttribute("aria-label", uiLanguage === "en" ? "MHEARD help" : "Pomoc MHEARD");
    help.innerHTML = helpIcon;
    head.appendChild(help);
    refreshPanelMHeard();
  }

  function setupMHeardHelp() {
    const modal = $("mheardHelpModal");
    if (!modal || modal.dataset.bound) return;
    modal.dataset.bound = "true";
    document.querySelectorAll("[data-mheard-help]").forEach((button) => {
      button.innerHTML = helpIcon;
      button.onclick = () => { applyDynamicLanguage(); modal.hidden = false; };
    });
    const hide = () => { modal.hidden = true; };
    $("mheardHelpClose").onclick = hide;
    $("mheardHelpOk").onclick = hide;
    modal.onclick = (event) => { if (event.target === modal) hide(); };
    applyDynamicLanguage();
  }

  async function refreshPanelMHeard() {
    try {
      const list = await fetch("/api/mheard", { cache: "no-store" }).then((r) => r.json());
      list.forEach((e) => {
        if (typeof rememberActivity === "function") rememberActivity(e.callsign, e.last_seen);
      });

      const el = $("mheardList");
      if (!el) return;

      el.innerHTML = list.length
        ? list.slice(0, 60).map((e) => `
          <div class="heard-row${e.source_type === "reported" ? " uprd-reported" : (e.operator_present !== undefined ? " uprd-sender" : "")}" data-call="${esc(e.callsign)}" data-port="${esc(e.port)}" data-via="${esc(e.via || "")}" title="${uiLanguage === "en" ? "Double-click to connect" : "Dwuklik: połącz"}">
            <strong>${typeof activityLED === "function" ? activityLED(e.callsign, e.last_seen) : ""}${e.operator_present !== undefined && typeof operatorPresenceIcon === "function" ? operatorPresenceIcon(e.operator_present) : ""}${esc(e.callsign)}</strong>
            <span class="mheard-via">${e.via ? "via" : ""}</span>
            <time>${age(e.last_seen)}</time>
          </div>
        `).join("")
        : '<p class="empty-list">Brak odebranych stacji.</p>';

      el.querySelectorAll(".heard-row").forEach((row) => {
        row.ondblclick = () => window.connectRadioStation?.(row.dataset.call, row.dataset.port, row.dataset.via || "");
      });

      if (typeof updateActivityLEDs === "function") updateActivityLEDs();
      applyDynamicLanguage();
    } catch {
      // no-op
    }
  }

  function positions(nodes, root) {
    const result = new Map([[root, { x: 50, y: 50 }]]);
    const ring = nodes.filter((n) => n.callsign !== root);
    const radius = 36;

    ring.forEach((n, i) => {
      const angle = (i / Math.max(1, ring.length)) * Math.PI * 2 - Math.PI / 2;
      result.set(n.callsign, { x: 50 + Math.cos(angle) * radius, y: 50 + Math.sin(angle) * radius });
    });

    return result;
  }

  function applyMapTransform() {
    const canvas = $("topologyCanvas");
    if (canvas) canvas.style.transform = `translate(${state.map.x}px, ${state.map.y}px) scale(${state.map.scale})`;
    const zoom = $("mapZoomValue");
    if (zoom) zoom.textContent = `${Math.round(state.map.scale * 100)}%`;
  }

  function setupMapNavigation() {
    const graph = $("topologyGraph");
    if (!graph || graph.dataset.navigation) return;

    graph.dataset.navigation = "true";
    graph.addEventListener("wheel", (e) => {
      e.preventDefault();
      state.map.scale = Math.min(2.5, Math.max(0.5, state.map.scale * (e.deltaY < 0 ? 1.1 : 0.9)));
      applyMapTransform();
    }, { passive: false });

    graph.addEventListener("dblclick", (e) => {
      if (e.target.closest(".topology-node")) window.showPage?.("terminal");
    }, true);

    graph.addEventListener("pointerdown", (e) => {
      if (e.target.closest(".topology-node")) return;
      state.map.drag = true;
      state.map.startX = e.clientX;
      state.map.startY = e.clientY;
      state.map.originX = state.map.x;
      state.map.originY = state.map.y;
      graph.setPointerCapture(e.pointerId);
      graph.classList.add("dragging");
    });

    graph.addEventListener("pointermove", (e) => {
      if (!state.map.drag) return;
      state.map.x = state.map.originX + e.clientX - state.map.startX;
      state.map.y = state.map.originY + e.clientY - state.map.startY;
      applyMapTransform();
    });

    const stop = (e) => {
      state.map.drag = false;
      graph.classList.remove("dragging");
      if (e.pointerId !== undefined && graph.hasPointerCapture(e.pointerId)) {
        graph.releasePointerCapture(e.pointerId);
      }
    };

    graph.addEventListener("pointerup", stop);
    graph.addEventListener("pointercancel", stop);

    $("mapZoomIn").onclick = () => {
      state.map.scale = Math.min(2.5, state.map.scale * 1.15);
      applyMapTransform();
    };
    $("mapZoomOut").onclick = () => {
      state.map.scale = Math.max(0.5, state.map.scale / 1.15);
      applyMapTransform();
    };
    $("mapZoomReset").onclick = () => {
      state.map = { ...state.map, x: 0, y: 0, scale: 1 };
      applyMapTransform();
    };
  }

  function setupMapHover() {
    const graph = $("topologyGraph");
    if (!graph || graph.dataset.hover) return;

    graph.dataset.hover = "true";
    graph.addEventListener("mouseover", (e) => {
      const node = e.target.closest(".topology-node");
      if (!node) return;
      const locator = node.querySelector("small")?.textContent.trim();
      const interfaces = [...node.querySelectorAll(".tnc-underlines i")].map((i) => i.title).filter(Boolean);
      node.title = [
        locator && `Lokator: ${locator}`,
        interfaces.length && `Interfejsy: ${interfaces.join(", ")}`,
        `${uiLanguage === "en" ? "Status" : "Status"}: ${node.classList.contains("green") ? (uiLanguage === "en" ? "fresh" : "świeży") : node.classList.contains("yellow") ? (uiLanguage === "en" ? "stale" : "starszy") : (uiLanguage === "en" ? "outdated" : "nieaktualny")}`,
      ].filter(Boolean).join(" · ");
    });
  }

  function renderMap(s) {
    const graph = $("topologyGraph");
    if (!graph) return;

    const root = s.root || "LOCAL";
    const nodes = s.nodes && s.nodes.length
      ? s.nodes
      : [{ callsign: root, locator: "", interfaces: [], last_seen: new Date().toISOString() }];
    const edges = s.edges || [];
    const routes = s.routes || [];
    const pos = positions(nodes, root);
    const controls = graph.querySelector(".map-zoom");

    graph.replaceChildren();

    const canvas = document.createElement("div");
    canvas.id = "topologyCanvas";
    canvas.className = "topology-canvas";

    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("viewBox", "0 0 100 100");
    svg.setAttribute("preserveAspectRatio", "none");
    svg.className = "topology-edges";

    for (const e of edges) {
      const a = pos.get(e.from);
      const b = pos.get(e.to);
      if (!a || !b) continue;
      const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
      line.setAttribute("x1", a.x);
      line.setAttribute("y1", a.y);
      line.setAttribute("x2", b.x);
      line.setAttribute("y2", b.y);
      line.setAttribute("stroke", color(e.interface_id));
      line.classList.add(freshness(e.last_seen));
      svg.appendChild(line);
    }
    canvas.appendChild(svg);

    for (const n of nodes) {
      const p = pos.get(n.callsign);
      const button = document.createElement("button");
      const route = routes.find((r) => r.destination === n.callsign);

      button.className = `topology-node ${freshness(n.effective_seen || n.last_seen)} ${n.callsign === root ? "root" : ""}`;
      button.style.left = `${p.x}%`;
      button.style.top = `${p.y}%`;
      button.innerHTML = `
        <strong>${esc(n.callsign)}</strong>
        <small>${esc(n.locator || "")}</small>
        <span class="tnc-underlines">
          ${(n.interfaces || []).filter((x) => x !== "LOCAL").map((x) => `<i style="background:${color(x)}" title="${esc(x)}"></i>`).join("")}
        </span>
      `;
      button.ondblclick = () => {
        if (route && n.callsign !== root) window.connectRadioStation?.(n.callsign, route.tnc, (route.via || []).join(","));
      };
      canvas.appendChild(button);
    }

    graph.appendChild(canvas);
    if (controls) graph.appendChild(controls);
    applyMapTransform();
    setupMapNavigation();

    const routeList = $("mapRoutes");
    if (routeList) {
      routeList.innerHTML = routes.filter((r) => r.destination !== root).map((r) => `
        <div class="route-row"><strong>${esc(r.destination)}</strong><span>${esc(r.path.join(" → "))}</span><em>${esc(r.tnc)}</em></div>
      `).join("") || '<p class="empty-list">Brak znanych tras.</p>';
    }

    if ($("mapSummary")) $("mapSummary").textContent = uiLanguage === "en"
      ? `${nodes.length} stations · ${edges.length} relations · ${new Date(s.generated).toLocaleTimeString()}`
      : `${nodes.length} stacji · ${edges.length} relacji · ${new Date(s.generated).toLocaleTimeString()}`;
    applyDynamicLanguage();
  }

  function renderPorts() {
    const box = $("mapPorts");
    const legend = $("mapLegend");
    if (!box || !legend) return;

    box.innerHTML = '<label class="map-check-all"><input type="checkbox" data-map-port="*" checked> Wszystkie</label>'
      + state.ports.map((p) => `<label><input type="checkbox" data-map-port="${esc(p)}" checked><i class="map-port-color" style="background:${color(p)}"></i>${esc(p)}</label>`).join("");
    legend.innerHTML = state.ports.map((p) => `<span><i style="background:${color(p)}"></i>${esc(p)}</span>`).join("");
    applyDynamicLanguage();

    box.querySelectorAll("input").forEach((i) => {
      i.onchange = () => {
        if (i.dataset.mapPort === "*") {
          box.querySelectorAll('input:not([data-map-port="*"])').forEach((x) => {
            x.checked = i.checked;
          });
        }
        refreshMap();
      };
    });

  }

  async function refreshMap() {
    if (!featureFlags.map) return;
    try {
      const selected = [...document.querySelectorAll('#mapPorts input:not([data-map-port="*"]):checked')].map((i) => i.dataset.mapPort);
      const all = document.querySelector('#mapPorts input[data-map-port="*"]')?.checked !== false;
      const q = all || !selected.length ? "" : `?ports=${encodeURIComponent(selected.join(","))}`;
      const s = await fetch("/api/uprd" + q, { cache: "no-store" }).then((r) => r.json());
      renderMap(s);
    } catch {
      // no-op
    }
  }

  function setupHeaderStatusButton() {
    const button = $("sendBeacon");
    if (!button || button.dataset.activeStatusBound) return;
    button.dataset.activeStatusBound = "true";
    const send = async (label, success, failure) => {
      button.disabled = true;
      try {
        const response = await fetch("/api/beacon", { method: "POST" });
        if (!response.ok) throw new Error(await response.text());
        button.textContent = success();
        setTimeout(() => { button.textContent = label(); }, 1800);
      } catch (error) {
        alert(`${failure()}: ${error.message}`);
        button.textContent = label();
      } finally {
        button.disabled = false;
      }
    };
    const beaconLabel = () => uiLanguage === "en" ? "Send beacon" : "Wyślij beacon";
    button.textContent = beaconLabel();
    button.onclick = () => send(beaconLabel, () => uiLanguage === "en" ? "Beacon sent" : "Beacon wysłany", () => uiLanguage === "en" ? "Beacon was not sent" : "Nie wysłano beaconu");
  }

  function setupHistoryPanel() {
    const panel = document.querySelector(".history-panel");
    const head = panel?.querySelector(".side-head");
    if (!panel || !head || $("historyPanelClose")) return;
    const close = document.createElement("button");
    close.id = "historyPanelClose";
    close.className = "history-panel-close";
    close.type = "button";
    close.textContent = "×";
    close.title = "Ukryj historię rozmów";
    close.setAttribute("aria-label", close.title);
    head.appendChild(close);
    const setVisible = (visible) => {
      document.body.classList.toggle("history-panel-hidden", !visible);
      localStorage.setItem("historyPanelVisible", visible ? "on" : "off");
    };
    close.onclick = () => setVisible(false);
    setVisible(localStorage.getItem("historyPanelVisible") !== "off");
  }

  async function connectFromHistory(callsign) {
    const call = String(callsign || "").trim().toUpperCase();
    if (!call) return;
    try {
      const list = await fetch("/api/mheard?mode=direct", { cache: "no-store" }).then((response) => response.json());
      const heard = (Array.isArray(list) ? list : []).find((entry) => String(entry.callsign || "").trim().toUpperCase() === call && entry.port);
      if (heard) {
        setMode("tnc");
        connectRadioStation(call, heard.port, heard.via || "");
        return;
      }
    } catch {
      // Fall through to manual connection when MHeard is unavailable.
    }
    setMode("tnc");
    $("target").value = call;
    const bar = document.querySelector(".connection-bar");
    bar?.classList.remove("history-connection-attention");
    void bar?.offsetWidth;
    bar?.classList.add("history-connection-attention");
    $("target").focus();
  }

  function setupHistoryConnection() {
    const originalRefreshHistory = refreshHistory;
    refreshHistory = async () => {
      await originalRefreshHistory();
      document.querySelectorAll(".history-row").forEach((row) => {
        row.ondblclick = (event) => {
          event.preventDefault();
          connectFromHistory(row.dataset.station);
        };
      });
    };
    refreshHistory();
  }

  function removeHeaderPreferences() {
    $("themeToggle")?.remove();
    $("soundControl")?.remove();
  }

  function setupAppearanceConfig() {
    const tabs = document.querySelector(".config-tabs");
    const panels = document.querySelector(".config-panels");
    if (!tabs || !panels || $("configAppearanceTab")) return;
    const tab = document.createElement("button");
    tab.id = "configAppearanceTab";
    tab.textContent = uiLanguage === "en" ? "Appearance & sounds" : "Wygląd i dźwięki";
    tabs.insertBefore(tab, $("configNetworkTab") || null);
    const panel = document.createElement("div");
    panel.id = "appearanceConfigPanel";
    panel.className = "settings-panel";
    panel.hidden = true;
    panel.innerHTML = `<div class="settings-card appearance-settings-card"><h3>${uiLanguage === "en" ? "Appearance and sounds" : "Wygląd i dźwięki"}</h3><div class="appearance-settings-list"><label class="appearance-setting-row"><span>${uiLanguage === "en" ? "Show conversation history" : "Pokaż historię rozmów"}</span><input id="historyPanelEnabled" type="checkbox"></label><label class="appearance-setting-row"><span>${uiLanguage === "en" ? "Theme" : "Motyw"}</span><select id="appearanceTheme"><option value="dark">${uiLanguage === "en" ? "Dark" : "Ciemny"}</option><option value="light">${uiLanguage === "en" ? "Light" : "Jasny"}</option></select></label><label class="appearance-setting-row"><span>${uiLanguage === "en" ? "Enable sound notifications" : "Włącz powiadomienia dźwiękowe"}</span><input id="appearanceSoundEnabled" type="checkbox"></label><label class="appearance-setting-row appearance-volume-row"><span><b>${uiLanguage === "en" ? "Notification volume" : "Głośność powiadomień"}</b><small>${uiLanguage === "en" ? "One tone indicates a message; three tones indicate an incoming connection." : "Jeden sygnał oznacza wiadomość, trzy sygnały — połączenie przychodzące."}</small></span><span class="appearance-volume-control"><input id="appearanceSoundVolume" type="range" min="0" max="100" step="1"><button id="soundHelpButton" class="help-button" type="button" aria-label="${uiLanguage === "en" ? "Sound notification help" : "Pomoc powiadomień dźwiękowych"}">${helpIcon}</button></span></label></div></div><div id="soundHelpModal" class="uprd-help-modal" hidden role="dialog" aria-modal="true"><div class="uprd-help-content"><div class="settings-card-heading"><h3>${uiLanguage === "en" ? "Sound notifications" : "Powiadomienia dźwiękowe"}</h3><button id="soundHelpClose" class="help-button" type="button" aria-label="${uiLanguage === "en" ? "Close" : "Zamknij"}">×</button></div><p>${uiLanguage === "en" ? "A single tone is played for a new message. Three short tones are played for a new incoming connection. The volume slider applies to both notification types." : "Przy nowej wiadomości odtwarzany jest jeden sygnał. Przy nowym połączeniu przychodzącym odtwarzane są trzy krótkie sygnały. Suwak głośności dotyczy obu rodzajów powiadomień."}</p><button id="soundHelpOk" class="secondary" type="button">OK</button></div></div>`;
    panels.appendChild(panel);
    tab.onclick = () => showConfigPart("appearance");
    $("historyPanelEnabled").checked = localStorage.getItem("historyPanelVisible") !== "off";
    $("historyPanelEnabled").onchange = () => {
      const visible = $("historyPanelEnabled").checked;
      document.body.classList.toggle("history-panel-hidden", !visible);
      localStorage.setItem("historyPanelVisible", visible ? "on" : "off");
    };
    $("appearanceTheme").value = document.documentElement.dataset.theme || "dark";
    $("appearanceTheme").onchange = () => setTheme($("appearanceTheme").value);
    $("appearanceSoundEnabled").checked = soundEnabled;
    $("appearanceSoundEnabled").onchange = () => { soundEnabled = $("appearanceSoundEnabled").checked; localStorage.setItem("terminalSound", soundEnabled ? "on" : "off"); updateSoundButton(); };
    $("appearanceSoundVolume").value = String(Math.round(soundVolume * 100));
    $("appearanceSoundVolume").oninput = (event) => { soundVolume = Math.max(0, Math.min(1, Number(event.target.value) / 100)); localStorage.setItem("terminalVolume", String(soundVolume)); updateSoundButton(); };
    $("appearanceSoundVolume").onchange = () => { if (soundEnabled) soundTone(660, 0, .1); };
    $("soundHelpButton").onclick = () => { $("soundHelpModal").hidden = false; };
    $("soundHelpClose").onclick = $("soundHelpOk").onclick = () => { $("soundHelpModal").hidden = true; };
  }

  function setup() {
    addNav();
    addConfig();
    setupHistoryPanel();
    setupHistoryConnection();
    setupAppearanceConfig();
    removeHeaderPreferences();
    setupHeaderStatusButton();
    document.documentElement.dataset.singleWebSession = "true";
    const monitorRenderer = () => {
      const list = $("monitorList");
      if (!list) return;
      list.innerHTML = monitorEntries.length ? monitorEntries.map((e) => {
        const content = e.content || "—";
        return `<div class="monitor-row"><time>${esc(new Date(e.time).toLocaleTimeString())}</time><b class="${esc(String(e.direction || "").toLowerCase())}">${esc(e.direction || "")}</b><span>${esc(e.port || "")}</span><div class="monitor-route"><strong>${esc(e.source || "")} → ${esc(e.destination || "")}</strong>${e.via ? `<span class="monitor-via">via ${esc(e.via)}</span>` : ""}</div><span class="monitor-content" title="${esc(content)}">${esc(content)}</span><em>${esc(e.type || "")} · ${Number(e.bytes) || 0} B</em></div>`;
      }).join("") : '<p class="empty-list">Brak ramek.</p>';
    };
    if (typeof renderMonitorList === "function") renderMonitorList = monitorRenderer;
    const checkWebSession = async () => {
      try {
        const response = await fetch("/api/status", { cache: "no-store" });
        if (response.status === 401) location.replace("/login.html");
      } catch {
        // Temporary network failures do not log the operator out.
      }
    };
    setInterval(checkWebSession, 2000);
    setupExperimentalControls();
    if (typeof applyUILanguage === "function") applyUILanguage(uiLanguage);
    setupPanelMHeard();
    setupMHeardHelp();
    if (featureFlags.map) {
      setupMapHover();
    }
    syncNavigation();
    wrapPages();

    fetch("/api/status", { cache: "no-store" })
      .then((r) => r.json())
      .then((s) => {
        state.ports = (s.ports || s.tncs || []).map((p) => typeof p === "string" ? p : (p.id || p.ID || p.name)).filter(Boolean);
        renderPorts();
      })
      .catch(() => {});

    setInterval(() => {
      if (state.page === "map") refreshMap();
      refreshPanelMHeard();
    }, 2000);
  }

  setup();
})();
