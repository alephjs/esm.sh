import { assertEquals, assertStringIncludes } from "https://deno.land/std@0.220.0/assert/mod.ts";

Deno.test("issue #705", async () => {
  const dts = await fetch(
    `http://localhost:8080/shikiji@0.3.3/dist/index.d.mts`,
  ).then((res) => res.text());
  const { default: nord } = await import(
    `http://localhost:8080/shikiji@0.3.3/es2022/dist/themes/nord.js`
  );
  assertStringIncludes(dts, "'./types/types.d.mts'");
  assertEquals(nord.name, "nord");
});
