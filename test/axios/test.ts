import { assertEquals } from "https://deno.land/std@0.220.0/assert/mod.ts";

import axios from "http://localhost:8080/axios@1.3.4";

Deno.test("axios", async () => {
  const res = await axios.get("http://localhost:8080/status.json");
  assertEquals(typeof res.data.version, "number");
});
