<script lang="ts">
  import SvelteLogo from "./components/Logo.svelte"
</script>

<div class="center-box absolute bg">
  <SvelteLogo></SvelteLogo>
</div>

<div class="center-box relative">
  <h1>esm.sh</h1>
  <p class="desc">
    The <strong>no-build</strong> cdn for modern web development.
  </p>
  <div class="links">
    <a href="https://esm.sh" target="_blank" title="Website">
      <img src="./assets/globe.svg" alt="Website">
    </a>
    <a href="https://bsky.app/profile/esm.sh" target="_blank" title="Bluesky">
      <img src="./assets/bluesky.svg" alt="Bluesky">
    </a>
    <a href="https://github.com/esm-dev/esm.sh" target="_blank" title="Github">
      <img src="./assets/github.svg" alt="Github">
    </a>
  </div>
</div>

<style>
  h1 {
    color: #ff4000;
  }
</style>
