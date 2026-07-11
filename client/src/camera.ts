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
}
