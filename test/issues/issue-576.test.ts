import { assertEquals } from "jsr:@std/assert";

Deno.test("issue #576", async () => {
  const res = await fetch(`http://localhost:8080/dedent@0.7.0`);
  const tsHeader = res.headers.get("x-typescript-types");
  res.body?.cancel();
  assertEquals(tsHeader, `http://localhost:8080/@types/dedent@~0.7.2/index.d.ts`);
});
