import { mount } from "svelte";
import App from "./App.svelte";

const root = document.getElementById("root")!;
// remove loading placeholder
root.replaceChildren();
// mount the app
mount(App, { target: root });
