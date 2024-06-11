import { assertEquals } from "https://deno.land/std@0.220.0/assert/mod.ts";

import { Location } from "http://localhost:8080/@aws-sdk/client-location@3.48.0";

Deno.test("issue #601", async () => {
  assertEquals(typeof Location, "function");
});
