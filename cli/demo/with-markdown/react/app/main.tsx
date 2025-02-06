import { createRoot } from "react-dom/client";
import About from "./about.md?jsx";
import "./app.css";

createRoot(document.getElementById("root")!).render(<About />);
