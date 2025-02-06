import { mount } from "svelte";
import About from "./about.md?svelte";
import "./app.css";

mount(About, { target: document.getElementById("root")! });
