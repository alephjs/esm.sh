import { assertEquals } from "https://deno.land/std@0.220.0/assert/mod.ts";

import mod from "http://localhost:8080/@rollup/plugin-commonjs@11.1.0";

Deno.test("issue #392", () => {
  assertEquals(typeof mod, "function");
});
