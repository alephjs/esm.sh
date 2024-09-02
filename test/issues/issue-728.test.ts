import { assertEquals } from "jsr:@std/assert";

Deno.test("issue #728", async () => {
  const res2 = await fetch(
    `http://localhost:8080/@wooorm/starry-night@3.0.0/es2022/source.css.js`,
  );
  res2.body?.cancel();
  assertEquals(
    res2.headers.get("content-type"),
    "application/javascript; charset=utf-8",
  );
});
