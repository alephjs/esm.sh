import { assertEquals } from "https://deno.land/std@0.220.0/assert/mod.ts";

import { blue } from "http://localhost:8080/@twind/preset-tailwind@1.1.4/colors";

Deno.test("issue #527", () => {
  assertEquals(typeof blue, "object");
});
