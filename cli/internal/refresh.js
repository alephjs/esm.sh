// Fast-refresh for React!
// @see https://github.com/facebook/react/issues/16604#issuecomment-528663101

import RefreshRuntime from "https://esm.sh/react-refresh@0.14.0/runtime";

let timer;

const __REFRESH_RUNTIME__ = {
  register: (specifier) => {
    return (type, id) => {
      RefreshRuntime.register(type, specifier + " " + id);
    };
  },
  sign: RefreshRuntime.createSignatureFunctionForTransform,
};
const __REFRESH__ = () => {
  if (timer !== null) {
    clearTimeout(timer);
  }
  timer = setTimeout(() => {
    timer = null;
    RefreshRuntime.performReactRefresh();
  }, 30);
};

RefreshRuntime.injectIntoGlobalHook(globalThis);
globalThis.$RefreshReg$ = () => {};
globalThis.$RefreshSig$ = () => type => type;

export { __REFRESH__, __REFRESH_RUNTIME__ };
