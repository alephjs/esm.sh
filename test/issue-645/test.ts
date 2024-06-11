import { assertEquals, assertStringIncludes } from "https://deno.land/std@0.220.0/assert/mod.ts";

Deno.test("issue #645", async () => {
  const res = await fetch(
    `http://localhost:8080/node-releases@2.0.12/deno/data/release-schedule/release-schedule.json.js`,
    { redirect: "manual" },
  );
  assertEquals(res.status, 200);
  assertEquals(
    res.headers.get("content-type")!,
    "application/javascript; charset=utf-8",
  );
  assertStringIncludes(await res.text(), "export default ");
});
