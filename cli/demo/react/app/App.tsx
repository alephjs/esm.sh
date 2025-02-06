import { ReactLogo } from "./components/Logo.tsx";

export function App() {
  return (
    <>
      <div className="center-box absolute bg">
        <ReactLogo />
      </div>
      <div className="center-box relative">
        <h1 style={{ color: "#61DAFB" }}>esm.sh</h1>
        <p className="desc">
          The <strong>no-build</strong> cdn for modern web development.
        </p>
        <div className="links">
          <a href="https://esm.sh" target="_blank" title="Website">
            <img src="./assets/globe.svg" alt="Website" />
          </a>
          <a href="https://bsky.app/profile/esm.sh" target="_blank" title="Bluesky">
            <img src="./assets/bluesky.svg" alt="Bluesky" />
          </a>
          <a href="https://github.com/esm-dev/esm.sh" target="_blank" title="Github">
            <img src="./assets/github.svg" alt="Github" />
          </a>
        </div>
      </div>
    </>
  );
}
