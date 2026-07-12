// Wheel zoom and drag pan for the map canvas, done entirely with a CSS
// transform so click math (getBoundingClientRect based) needs no changes.
const MIN_ZOOM = 1;
const MAX_ZOOM = 8;
const DRAG_THRESHOLD_PX = 5;

let zoom = 1;
let tx = 0;
let ty = 0;
let dragging = false;
let dragMoved = 0;
let lastX = 0;
let lastY = 0;
let suppressClick = false;

// consumePan reports and clears the swallow flag set by a real drag, so
// the click that ends a pan never selects an entity or places a torch.
export function consumePan(): boolean {
  const s = suppressClick;
  suppressClick = false;
  return s;
}

export function initCamera(canvas: HTMLCanvasElement) {
  const map = document.getElementById("map")!;
  canvas.style.transformOrigin = "0 0";

  const apply = () => {
    // clamp so the scaled canvas always covers its original fitted box
    const minX = canvas.offsetWidth * (1 - zoom);
    const minY = canvas.offsetHeight * (1 - zoom);
    tx = Math.min(0, Math.max(minX, tx));
    ty = Math.min(0, Math.max(minY, ty));
    canvas.style.transform = `translate(${tx}px, ${ty}px) scale(${zoom})`;
    map.classList.toggle("zoomed", zoom > 1);
  };

  map.addEventListener(
    "wheel",
    (ev) => {
      ev.preventDefault();
      const r = canvas.getBoundingClientRect();
      const px = ev.clientX - r.left;
      const py = ev.clientY - r.top;
      const next = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom * Math.exp(-ev.deltaY * 0.0015)));
      // keep the world point under the cursor fixed while scaling
      tx = px + tx - (px / zoom) * next;
      ty = py + ty - (py / zoom) * next;
      zoom = next;
      apply();
    },
    { passive: false },
  );

  map.addEventListener("mousedown", (ev) => {
    if (ev.button !== 0 || zoom <= 1) return;
    dragging = true;
    dragMoved = 0;
    lastX = ev.clientX;
    lastY = ev.clientY;
    map.classList.add("panning");
  });
  window.addEventListener("mousemove", (ev) => {
    if (!dragging) return;
    tx += ev.clientX - lastX;
    ty += ev.clientY - lastY;
    dragMoved += Math.abs(ev.clientX - lastX) + Math.abs(ev.clientY - lastY);
    lastX = ev.clientX;
    lastY = ev.clientY;
    apply();
  });
  window.addEventListener("mouseup", () => {
    if (!dragging) return;
    dragging = false;
    map.classList.remove("panning");
    if (dragMoved > DRAG_THRESHOLD_PX) suppressClick = true;
  });

  // touch: one finger pans, two fingers pinch-zoom around their midpoint;
  // taps still land as synthesized clicks (#map has touch-action: none)
  let pinchDist = 0;
  let midX = 0;
  let midY = 0;
  const tDist = (ts: TouchList) => Math.hypot(ts[0].clientX - ts[1].clientX, ts[0].clientY - ts[1].clientY);

  map.addEventListener(
    "touchstart",
    (ev) => {
      if (ev.touches.length === 1) {
        dragging = true;
        dragMoved = 0;
        lastX = ev.touches[0].clientX;
        lastY = ev.touches[0].clientY;
      } else if (ev.touches.length === 2) {
        dragging = false;
        pinchDist = tDist(ev.touches);
        midX = (ev.touches[0].clientX + ev.touches[1].clientX) / 2;
        midY = (ev.touches[0].clientY + ev.touches[1].clientY) / 2;
        suppressClick = true; // a pinch never selects or places
      }
    },
    { passive: true },
  );

  map.addEventListener(
    "touchmove",
    (ev) => {
      ev.preventDefault();
      if (ev.touches.length === 1 && dragging) {
        const t = ev.touches[0];
        tx += t.clientX - lastX;
        ty += t.clientY - lastY;
        dragMoved += Math.abs(t.clientX - lastX) + Math.abs(t.clientY - lastY);
        lastX = t.clientX;
        lastY = t.clientY;
        if (dragMoved > DRAG_THRESHOLD_PX) suppressClick = true;
        apply();
      } else if (ev.touches.length === 2) {
        const mx = (ev.touches[0].clientX + ev.touches[1].clientX) / 2;
        const my = (ev.touches[0].clientY + ev.touches[1].clientY) / 2;
        const nd = tDist(ev.touches);
        if (pinchDist > 0) {
          const r = canvas.getBoundingClientRect();
          const px = mx - r.left;
          const py = my - r.top;
          const next = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom * (nd / pinchDist)));
          tx = px + tx - (px / zoom) * next;
          ty = py + ty - (py / zoom) * next;
          zoom = next;
        }
        tx += mx - midX;
        ty += my - midY;
        pinchDist = nd;
        midX = mx;
        midY = my;
        apply();
      }
    },
    { passive: false },
  );

  map.addEventListener("touchend", (ev) => {
    if (ev.touches.length === 0) {
      dragging = false;
    } else if (ev.touches.length === 1) {
      // a pinch collapsed to one finger: continue as a pan, never a tap
      dragging = true;
      dragMoved = DRAG_THRESHOLD_PX + 1;
      lastX = ev.touches[0].clientX;
      lastY = ev.touches[0].clientY;
      pinchDist = 0;
    }
  });
}
