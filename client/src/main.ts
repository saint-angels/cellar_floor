import { connect } from "./net";
import { startRender, tileFromPixel } from "./render";
import { initFx } from "./fx";
import { initUI } from "./ui";

const canvas = document.getElementById("canvas") as HTMLCanvasElement;
startRender(canvas);
connect();
initFx();
initUI(canvas, tileFromPixel);
