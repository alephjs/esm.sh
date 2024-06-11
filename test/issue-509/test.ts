import { assertStringIncludes } from "https://deno.land/std@0.220.0/assert/mod.ts";

Deno.test("issue #509", async () => {
  const res = await fetch("http://localhost:8080/react@18.2.0", {
    headers: {
      "User-Agent": "HeadlessChrome/109",
    },
  });
  const text = await res.text();
  assertStringIncludes(text, "/es2022/");
});
