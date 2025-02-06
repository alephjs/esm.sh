// see https://link.vuejs.org/feature-flags.
Object.assign(globalThis, {
  __VUE_OPTIONS_API__: true,
  __VUE_PROD_DEVTOOLS__: false,
  __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false,
});

export default {
  updateStyle(id, styles) {
    const head = globalThis.document.head;
    const existing = globalThis.document.querySelectorAll(`style[data-v-${id}]`);
    for (const style of existing) {
      head.removeChild(style);
    }
    for (const css of styles) {
      head.insertAdjacentHTML("beforeend", `<style data-v-${id}>${css}</style>`);
    }
  },
};
