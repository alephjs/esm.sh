<script setup lang="ts">
  import VueLogo from "./components/Logo.vue"
</script>

<template>
  <div class="center-box absolute op15">
    <VueLogo></VueLogo>
  </div>
  <div class="center-box relative">
    <h1 class="font-sans text-5xl fw500 text-primary select-none">esm.sh</h1>
    <p class="font-sans text-lg fw400 text-gray-400 text-center">
      The <span class="fw600">no-build</span> cdn for modern web development.
    </p>
    <div class="flex justify-center gap-3 mt2 text-2xl all:transition-300">
      <a class="logo i-tabler-world" href="https://esm.sh" target="_blank" title="Website"></a>
      <a class="logo i-tabler-brand-bluesky" href="https://bsky.app/profile/esm.sh" target="_blank" title="Bluesky"></a>
      <a class="logo i-tabler-brand-github" href="https://github.com/esm-dev/esm.sh" target="_blank" title="Github"></a>
    </div>
  </div>
</template>
