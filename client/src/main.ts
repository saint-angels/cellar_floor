import { connect } from "./net";
import { startRender, tileFromPixel } from "./render";
import { initFx } from "./fx";
import { initCamera } from "./camera";
import { initUI } from "./ui";

const canvas = document.getElementById("canvas") as HTMLCanvasElement;
startRender(canvas);
connect();
initFx();
initCamera(canvas);
initUI(canvas, tileFromPixel);
