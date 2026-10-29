// Mapa da página inicial: pinos por categoria, lista agrupada por dia e filtros sem recarregar.
(function () {
  const el = document.getElementById("map");
  const list = document.getElementById("list");
  const form = document.getElementById("filters");
  const count = document.getElementById("count");
  if (!el || !window.L) return;

  const desktop = () => window.innerWidth > 860;
  const map = L.map(el, { zoomControl: false, scrollWheelZoom: true })
    .setView([+el.dataset.lat, +el.dataset.lng], 13);
  L.control.zoom({ position: "topright" }).addTo(map);
  // Tiles padrão do OpenStreetMap; o tom escuro vem de um filtro CSS (.map-dark), sem chave de API.
  L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    className: "map-dark",
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
  }).addTo(map);

  // O tamanho do mapa muda com o layout (celular/desktop, fontes carregando): avisa o Leaflet.
  new ResizeObserver(() => map.invalidateSize()).observe(el);

  const layer = L.layerGroup().addTo(map);
  const markers = new Map();
  const cta = list.querySelector(".panel-cta");

  function h(tag, attrs, ...children) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      if (k === "class") n.className = v;
      else n.setAttribute(k, v);
    }
    for (const c of children) if (c != null && c !== false) n.append(c);
    return n;
  }

  function icon(category, active) {
    return L.divIcon({
      className: "pin cat-" + category + (active ? " active" : ""),
      html: "<span></span>",
      iconSize: [28, 28],
      iconAnchor: [14, 14],
      popupAnchor: [0, -14],
    });
  }

  function popup(c) {
    return h("div", { class: "cat-" + c.category },
      h("span", { class: "cat-label" }, c.categoryLabel),
      h("h3", null, c.title),
      h("p", null, c.dayLabel + " · " + c.time + (c.sessions > 1 ? " · +" + (c.sessions - 1) + " sessões" : "")),
      h("p", null, c.place),
      h("p", null, c.free ? "Gratuito" : c.price),
      h("a", { class: "more", href: "/evento/" + c.slug }, "Ver detalhes →"));
  }

  function ticket(c) {
    return h("a", { class: "ticket cat-" + c.category, href: "/evento/" + c.slug, "data-id": c.id },
      h("span", { class: "ticket-stub" },
        h("span", { class: "ticket-wd" }, c.weekday),
        h("span", { class: "ticket-day" }, String(c.dayNum)),
        h("span", { class: "ticket-mo" }, c.month)),
      h("span", { class: "ticket-body" },
        h("span", { class: "cat-label" }, c.categoryLabel),
        h("span", { class: "ticket-title" }, c.title),
        h("span", { class: "ticket-place" }, c.place),
        h("span", { class: "ticket-meta" },
          h("span", { class: "ticket-time" }, c.time),
          c.free ? h("span", { class: "ticket-free" }, "Gratuito") : h("span", { class: "ticket-price" }, c.price),
          c.sessions > 1 && h("span", { class: "ticket-more" }, "+" + c.sessions + " sessões"))));
  }

  let active = null;
  function highlight(id, pan) {
    if (active != null && markers.has(active)) {
      const m = markers.get(active);
      m.setIcon(icon(m.options.category, false));
      m.setZIndexOffset(0);
    }
    list.querySelectorAll(".ticket.active").forEach((n) => n.classList.remove("active"));
    active = id;
    const m = markers.get(id);
    if (!m) return;
    m.setIcon(icon(m.options.category, true));
    m.setZIndexOffset(1000);
    const node = list.querySelector('.ticket[data-id="' + id + '"]');
    if (node) node.classList.add("active");
    if (pan) {
      map.flyTo(m.getLatLng(), Math.max(map.getZoom(), 15), { duration: 0.5 });
      m.openPopup();
    }
  }

  function fit(cards) {
    if (!cards.length) return;
    // No desktop o painel cobre a esquerda do mapa: empurra os pinos para a parte visível.
    const opts = desktop()
      ? { paddingTopLeft: [490, 60], paddingBottomRight: [60, 60], maxZoom: 15 }
      : { padding: [40, 40], maxZoom: 15 };
    if (cards.length === 1) {
      map.setView([cards[0].lat, cards[0].lng], 15);
      if (desktop()) map.panBy([-220, 0], { animate: false });
      return;
    }
    map.fitBounds(L.latLngBounds(cards.map((c) => [c.lat, c.lng])), opts);
  }

  function draw(cards) {
    layer.clearLayers();
    markers.clear();
    // Vários eventos no mesmo espaço: abre os pinos num pequeno círculo para não ficarem empilhados.
    const sameSpot = new Map();
    for (const c of cards) {
      const key = c.lat.toFixed(5) + "," + c.lng.toFixed(5);
      const n = sameSpot.get(key) || 0;
      sameSpot.set(key, n + 1);
      const angle = n * 2.4;
      const r = n ? 0.00022 : 0;
      const m = L.marker([c.lat + r * Math.sin(angle), c.lng + r * Math.cos(angle)], {
        icon: icon(c.category), category: c.category, title: c.title,
      }).bindPopup(() => popup(c));
      m.on("click", () => {
        highlight(c.id, false);
        const node = list.querySelector('.ticket[data-id="' + c.id + '"]');
        if (node && desktop()) node.scrollIntoView({ block: "nearest", behavior: "smooth" });
      });
      m.addTo(layer);
      markers.set(c.id, m);
    }
  }

  function renderList(cards) {
    list.replaceChildren();
    count.textContent = cards.length + (cards.length === 1 ? " evento no mapa" : " eventos no mapa");
    if (!cards.length) {
      list.append(h("div", { class: "empty" },
        h("p", null, "Nenhum evento nesse filtro."),
        h("p", null, h("a", { href: "/eventos/novo" }, "Conhece algum? Divulgue aqui."))));
    }
    let day = null, group = null;
    for (const c of cards) {
      if (c.dayKey !== day) {
        day = c.dayKey;
        list.append(h("div", { class: "day-head" + (c.dayLabel === "Hoje" ? " is-today" : "") }, c.dayLabel));
        group = h("div", { class: "tickets" });
        list.append(group);
      }
      group.append(ticket(c));
    }
    if (cta) list.append(cta);
  }

  // No desktop, passar o mouse no ingresso destaca o pino; no celular o toque abre a página.
  list.addEventListener("mouseover", (ev) => {
    const node = ev.target.closest(".ticket");
    if (node && desktop()) highlight(+node.dataset.id, false);
  });

  let controller;
  async function refresh() {
    const params = new URLSearchParams(new FormData(form));
    for (const [k, v] of [...params]) if (!v) params.delete(k);
    history.replaceState(null, "", params.toString() ? "?" + params : location.pathname);
    controller?.abort();
    controller = new AbortController();
    list.setAttribute("aria-busy", "true");
    try {
      const res = await fetch("/api/eventos?" + params, { signal: controller.signal });
      const cards = await res.json();
      renderList(cards);
      draw(cards);
      fit(cards);
    } catch (e) {
      if (e.name !== "AbortError") console.error(e);
    } finally {
      list.removeAttribute("aria-busy");
    }
  }
  form.addEventListener("change", refresh);
  form.addEventListener("submit", (ev) => { ev.preventDefault(); refresh(); });

  const initial = JSON.parse(el.dataset.cards || "[]");
  renderList(initial);
  draw(initial);
  fit(initial);
})();
