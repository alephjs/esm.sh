import { assertEquals, assertStringIncludes } from "https://deno.land/std@0.220.0/assert/mod.ts";

Deno.test("issue #711", async () => {
  const res = await fetch("http://localhost:8080/@pyscript/core@0.1.5/core.js", {
    headers: {
      "User-Agent":
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.3 Safari/605.1.15",
    },
  });
  await res.body?.cancel();
  const buildId = res.headers.get("x-esm-path")!;
  assertStringIncludes(buildId, "/es2021/");
  const res2 = await fetch(`http://localhost:8080/${buildId}`);
  res2.body?.cancel();
  assertEquals(res2.headers.get("content-type"), "application/javascript; charset=utf-8");
});
