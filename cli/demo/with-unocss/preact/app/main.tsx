import { render } from "preact";
import { App } from "./App.tsx";

const root = document.getElementById("root")!;
// remove loading placeholder
root.replaceChildren();
// render the app
render(<App />, root);
