// Mapa pequeno e fixo das páginas de evento e de espaço.
(function () {
  const el = document.getElementById("minimap");
  if (!el || !window.L) return;
  const pos = [+el.dataset.lat, +el.dataset.lng];
  const map = L.map(el, { scrollWheelZoom: false, dragging: !L.Browser.mobile, zoomControl: false }).setView(pos, 16);
  new ResizeObserver(() => map.invalidateSize()).observe(el);
  L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    className: "map-soft",
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
  }).addTo(map);
  L.marker(pos, {
    icon: L.divIcon({ className: "pin light cat-" + el.dataset.category, html: "<span></span>", iconSize: [28, 28], iconAnchor: [14, 14] }),
  }).addTo(map);
})();
