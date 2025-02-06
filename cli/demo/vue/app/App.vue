<script setup lang="ts">
  import VueLogo from "./components/Logo.vue"
</script>

<template>
  <div class="center-box absolute bg">
    <VueLogo></VueLogo>
  </div>
  <div class="center-box relative">
    <h1>esm.sh</h1>
    <p class="desc">
      The <strong>no-build</strong> cdn for modern web development.
    </p>
    <div class="links">
      <a href="https://esm.sh" target="_blank" title="Website">
        <img src="../assets/globe.svg">
      </a>
      <a href="https://bsky.app/profile/esm.sh" target="_blank" title="Bluesky">
        <img src="../assets/bluesky.svg">
      </a>
      <a href="https://github.com/esm-dev/esm.sh" target="_blank" title="Github">
        <img src="../assets/github.svg">
      </a>
    </div>
  </div>
</template>

<style scoped>
  h1 {
    color: #42b883;
  }
</style>
