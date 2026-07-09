import { connect } from "./net";
import { startRender, tileFromPixel } from "./render";
import { world } from "./world";
import { initUI } from "./ui";

const canvas = document.getElementById("canvas") as HTMLCanvasElement;
startRender(canvas);
connect();
initUI(canvas, tileFromPixel);
