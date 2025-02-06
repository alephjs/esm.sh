import { ReactLogo } from "./components/Logo.tsx";

export function App() {
  return (
    <>
      <div className="center-box absolute op15">
        <ReactLogo />
      </div>
      <div className="center-box relative">
        <h1 className="font-sans text-5xl fw500 text-primary select-none">esm.sh</h1>
        <p className="font-sans text-lg fw400 text-gray-400 text-center">
          The <span className="fw600">no-build</span> cdn for modern web development.
        </p>
        <div className="flex justify-center gap-3 mt2 text-2xl all:transition-300">
          <a className="logo i-tabler-world" href="https://esm.sh" target="_blank" title="Website" />
          <a className="logo i-tabler-brand-bluesky" href="https://bsky.app/profile/esm.sh" target="_blank" title="Bluesky" />
          <a className="logo i-tabler-brand-github" href="https://github.com/esm-dev/esm.sh" target="_blank" title="Github" />
        </div>
      </div>
    </>
  );
}
