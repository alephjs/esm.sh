import { assert } from "jsr:@std/assert";

import { minify } from "http://localhost:8080/terser";

Deno.test("terser", async () => {
  const code = "function add(first, second) { return first + second; }";
  const result = await minify(code, { sourceMap: true });
  assert(result.code === "function add(n,d){return n+d}");
  assert(JSON.parse(String(result.map)).names?.length === 3);
});
