import "./app.css";

const html = String.raw;

document.body.innerHTML = html`
<div class="center-box relative">
  <h1>esm.sh</h1>
  <p class="desc">
    The <strong>no-build</strong> cdn for modern web development.
  </p>
  <div class="links">
    <a href="https://esm.sh" target="_blank" title="Website">
      <img src="./assets/globe.svg">
    </a>
    <a href="https://bsky.app/profile/esm.sh" target="_blank" title="Bluesky">
      <img src="./assets/bluesky.svg">
    </a>
    <a href="https://github.com/esm-dev/esm.sh" target="_blank" title="Github">
      <img src="./assets/github.svg">
    </a>
  </div>
</div>
`;
