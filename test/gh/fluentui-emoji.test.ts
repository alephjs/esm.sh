import { assertEquals, assertStringIncludes } from "jsr:@std/assert";

Deno.test("github assets", async () => {
  const res = await fetch(
    "http://localhost:8080/gh/microsoft/fluentui-emoji/assets/Alien/Flat/alien_flat.svg",
  );
  assertEquals(res.status, 200);
  assertEquals(res.headers.get("content-type"), "image/svg+xml");
  assertStringIncludes(await res.text(), "<svg");
});
