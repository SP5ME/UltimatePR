(function () {
  const featureFlags = {
    uprd: false,
    map: false,
    mheardToggle: false,
  };

  const state = {
    page: "terminal",
    mheardMode: "direct",
    ports: [],
    positions: new Map(),
    rings: new Map(),
    colors: new Map(),
    map: { x: 0, y: 0, scale: 1, drag: false, startX: 0, startY: 0, originX: 0, originY: 0 },
  };

  const $ = (id) => document.getElementById(id);
  const applyDynamicLanguage = () => {
    if (typeof applyUILanguage === "function") applyUILanguage(uiLanguage);
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

    const card = document.createElement("div");
    card.id = "uprdConfigCard";
    card.className = "settings-card";
    card.innerHTML = `
      <h3>UPR Direct</h3>
      <p>UPRD wysyła krótki raport lokalnej wiedzy RF nad standardowym AX.25 UI. Funkcja jest niezależna od klasycznego beaconu.</p>
      <div class="field-grid">
        <label class="check"><input id="uprdEnabled" type="checkbox"> UPRD aktywne</label>
        <label>Interwał (minuty)<input id="uprdInterval" type="number" min="1" max="1440"></label>
        <label>Limit MHEARD (maks.)<input id="uprdLimit" type="number" min="1" max="10"></label>
      </div>
    `;

    card.hidden = !featureFlags.uprd;
    panel.appendChild(card);
    $("configUPRDirectTab")?.toggleAttribute("hidden", !featureFlags.uprd);

    const oldFill = window.fillApplication;
    if (typeof oldFill === "function") {
      window.fillApplication = (c) => {
        oldFill(c);
        const u = c.UPRD || {};
        $("uprdEnabled").checked = !!u.Enabled;
        $("uprdInterval").value = Math.max(1, Math.round((u.IntervalSeconds || 600) / 60));
        $("uprdLimit").value = u.MHeardLimit || 5;
      };
    }

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
    featureFlags.map = featureFlags.uprd;
    featureFlags.mheardToggle = featureFlags.uprd;

    const mapButton = $("navMap");
    if (mapButton) {
      mapButton.hidden = !featureFlags.map;
      mapButton.disabled = !featureFlags.map;
      mapButton.onclick = featureFlags.map ? () => open("map") : null;
    }
    $("configUPRDirectTab")?.toggleAttribute("hidden", !featureFlags.uprd);
    $("uprdConfigCard")?.toggleAttribute("hidden", !featureFlags.uprd);
    $("mheardPanelTabs")?.toggleAttribute("hidden", !featureFlags.mheardToggle);
    document.querySelector("#mheardView .segmented")?.toggleAttribute("hidden", !featureFlags.mheardToggle);
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
      featureFlags.uprd = master.checked && uprd.checked;
      list.hidden = !master.checked;
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
        <div class="heard-row" data-call="${esc(e.callsign)}" data-port="${esc(e.port)}" data-via="${esc(e.via || "")}">
          <strong>${esc(e.callsign)}</strong>
          <span class="mheard-via">${e.via ? "via " + esc(e.via) : esc(e.port)}</span>
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
      const list = await fetch(`/api/mheard?mode=${state.mheardMode}`, { cache: "no-store" }).then((r) => r.json());
      renderList(list, "mheardFullList");
    } catch {
      // no-op
    }
  }

  function setupPanelMHeard() {
    const head = document.querySelector(".mheard-panel .side-head");
    if (!head || $("mheardPanelTabs")) return;

    window.discoveryMHeardOwner = true;
    const small = head.querySelector("small");
    if (small) small.remove();

    const tabs = document.createElement("div");
    tabs.id = "mheardPanelTabs";
    tabs.className = "segmented compact";
    tabs.innerHTML = '<button id="mheardPanelDirect" class="active" type="button">Direct</button><button id="mheardPanelUprd" type="button">UPRD</button>';

    if (!featureFlags.mheardToggle) tabs.hidden = true;
    head.appendChild(tabs);

    state.mheardMode = "direct";
    refreshPanelMHeard();

    const load = (mode) => {
      state.mheardMode = mode;
      $("mheardPanelDirect").classList.toggle("active", mode === "direct");
      $("mheardPanelUprd").classList.toggle("active", mode === "uprd");
      refreshPanelMHeard();
    };

    $("mheardPanelDirect").onclick = () => load("direct");
    $("mheardPanelUprd").onclick = () => load("uprd");
  }

  async function refreshPanelMHeard() {
    try {
      const list = await fetch(`/api/mheard?mode=${state.mheardMode}`, { cache: "no-store" }).then((r) => r.json());
      list.forEach((e) => {
        if (typeof rememberActivity === "function") rememberActivity(e.callsign, e.last_seen);
      });

      const el = $("mheardList");
      if (!el) return;

      el.innerHTML = list.length
        ? list.slice(0, 60).map((e) => `
          <div class="heard-row" data-call="${esc(e.callsign)}" data-port="${esc(e.port)}" data-via="${esc(e.via || "")}" title="${uiLanguage === "en" ? "Double-click to connect" : "Dwuklik: połącz"}">
            <strong>${typeof activityLED === "function" ? activityLED(e.callsign, e.last_seen) : ""}${e.indirect ? "*" : ""}${esc(e.callsign)}</strong>
            <span class="mheard-via">${e.via ? "via " + esc(e.via) : ""}</span>
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

    const send = $("sendUprdNow");
    if (send && !send.dataset.bound) {
      send.dataset.bound = "true";
      send.onclick = async () => {
        send.disabled = true;
        try {
          const r = await fetch("/api/uprd/send", { method: "POST" });
          if (!r.ok) throw new Error(await r.text());
          send.textContent = uiLanguage === "en" ? "UPRD status sent" : "Status UPRD wysłany";
          setTimeout(() => { send.textContent = uiLanguage === "en" ? "Send UPRD status" : "Wyślij status UPRD"; }, 1800);
        } catch (e) {
          alert(`${uiLanguage === "en" ? "UPRD was not sent" : "Nie wysłano UPRD"}: ${e.message}`);
        } finally {
          send.disabled = false;
        }
      };
    }
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

  function setup() {
    addNav();
    addConfig();
    setupExperimentalControls();
    if (typeof applyUILanguage === "function") applyUILanguage(uiLanguage);
    setupPanelMHeard();
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
