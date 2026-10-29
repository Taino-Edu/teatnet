// Formulários de evento e de espaço: escolher o ponto no mapa, buscar endereço e montar sessões.
(function () {
  const $ = (id) => document.getElementById(id);

  // ---------- mapa para marcar o local ----------
  const pickerEl = $("picker");
  const latIn = $("lat");
  const lngIn = $("lng");
  const venueSel = $("venue");
  const custom = $("custom-place");
  let map, marker;

  function setPoint(lat, lng, zoom) {
    if (!map) return;
    const ll = [lat, lng];
    if (!marker) {
      marker = L.marker(ll, { draggable: true }).addTo(map);
      marker.on("dragend", () => {
        const p = marker.getLatLng();
        latIn.value = p.lat.toFixed(6);
        lngIn.value = p.lng.toFixed(6);
      });
    } else {
      marker.setLatLng(ll);
    }
    latIn.value = (+lat).toFixed(6);
    lngIn.value = (+lng).toFixed(6);
    map.setView(ll, zoom || Math.max(map.getZoom(), 16));
  }

  if (pickerEl && window.L) {
    map = L.map(pickerEl).setView([+pickerEl.dataset.lat, +pickerEl.dataset.lng], 13);
    new ResizeObserver(() => map.invalidateSize()).observe(pickerEl);
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      className: "map-soft",
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map);
    map.on("click", (ev) => {
      if (venueSel && venueSel.value) return; // espaço cadastrado: o ponto é o dele
      setPoint(ev.latlng.lat, ev.latlng.lng, map.getZoom());
    });
    if (latIn.value && lngIn.value) setPoint(+latIn.value, +lngIn.value, 16);
  }

  // Espaço cadastrado esconde os campos de "outro local" e fixa o pino no espaço.
  function syncVenue() {
    if (!venueSel) return;
    const opt = venueSel.selectedOptions[0];
    const isCustom = !venueSel.value;
    custom.hidden = !isCustom;
    if (marker) marker.dragging[isCustom ? "enable" : "disable"]();
    if (!isCustom && opt) setPoint(+opt.dataset.lat, +opt.dataset.lng, 16);
  }
  if (venueSel) {
    venueSel.addEventListener("change", () => {
      if (!venueSel.value) { latIn.value = ""; lngIn.value = ""; if (marker) { marker.remove(); marker = null; } }
      syncVenue();
    });
    syncVenue();
  }

  // ---------- busca de endereço ----------
  const geoBtn = $("geocode");
  const results = $("geo-results");
  if (geoBtn) {
    const address = $("address");
    async function search() {
      const q = address.value.trim();
      if (q.length < 3) return;
      geoBtn.disabled = true;
      geoBtn.textContent = "Buscando…";
      results.replaceChildren();
      try {
        const res = await fetch("/api/geocode?q=" + encodeURIComponent(q));
        if (!res.ok) throw new Error(await res.text());
        const found = await res.json();
        if (!found.length) {
          const li = document.createElement("li");
          li.className = "hint";
          li.style.padding = "10px 12px";
          li.textContent = "Não achamos esse endereço. Tente sem o número ou clique direto no mapa.";
          results.append(li);
        }
        for (const g of found) {
          const li = document.createElement("li");
          const b = document.createElement("button");
          b.type = "button";
          b.textContent = g.label;
          b.addEventListener("click", () => {
            setPoint(g.lat, g.lng, 17);
            results.hidden = true;
          });
          li.append(b);
          results.append(li);
        }
        results.hidden = false;
        if (found.length === 1) { setPoint(found[0].lat, found[0].lng, 17); }
      } catch (e) {
        const li = document.createElement("li");
        li.className = "hint";
        li.style.padding = "10px 12px";
        li.textContent = "Busca indisponível agora. Marque o ponto clicando no mapa.";
        results.append(li);
        results.hidden = false;
      } finally {
        geoBtn.disabled = false;
        geoBtn.textContent = "Buscar no mapa";
      }
    }
    geoBtn.addEventListener("click", search);
    address.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") { ev.preventDefault(); search(); }
    });
  }

  // ---------- sessões ----------
  const rows = $("sessions");
  const tpl = $("session-template");
  if (rows && tpl) {
    function addRow(date, time) {
      const node = tpl.content.firstElementChild.cloneNode(true);
      node.querySelector('[name="s_date"]').value = date || "";
      node.querySelector('[name="s_time"]').value = time || "";
      rows.append(node);
      return node;
    }
    function emptyRows() {
      return [...rows.querySelectorAll(".session-row")].filter(
        (r) => !r.querySelector('[name="s_date"]').value && !r.querySelector('[name="s_time"]').value);
    }

    $("add-session").addEventListener("click", () => {
      // Repete o horário da última sessão, que costuma ser igual.
      const times = rows.querySelectorAll('[name="s_time"]');
      const last = times.length ? times[times.length - 1].value : "";
      addRow("", last).querySelector('[name="s_date"]').focus();
    });
    rows.addEventListener("click", (ev) => {
      const btn = ev.target.closest("[data-remove-session]");
      if (!btn) return;
      btn.closest(".session-row").remove();
      if (!rows.children.length) addRow();
    });

    const season = $("season");
    $("toggle-season").addEventListener("click", () => { season.hidden = !season.hidden; });
    $("season-apply").addEventListener("click", () => {
      const from = $("season-from").value;
      const to = $("season-to").value;
      const time = $("season-time").value;
      const days = [...season.querySelectorAll(".weekdays input:checked")].map((i) => +i.value);
      if (!from || !to || !time || !days.length || to < from) {
        alert("Preencha o período, o horário e pelo menos um dia da semana.");
        return;
      }
      const existing = new Set([...rows.querySelectorAll(".session-row")].map(
        (r) => r.querySelector('[name="s_date"]').value + " " + r.querySelector('[name="s_time"]').value));
      emptyRows().forEach((r) => r.remove());
      // Datas em UTC para não sofrer com fuso ao avançar dia a dia.
      const d = new Date(from + "T00:00:00Z");
      const end = new Date(to + "T00:00:00Z");
      let added = 0;
      while (d <= end && added < 90) {
        if (days.includes(d.getUTCDay())) {
          const iso = d.toISOString().slice(0, 10);
          if (!existing.has(iso + " " + time)) { addRow(iso, time); added++; }
        }
        d.setUTCDate(d.getUTCDate() + 1);
      }
      if (!rows.children.length) addRow();
      season.hidden = true;
    });
  }
})();
