import { assertEquals } from "jsr:@std/assert";

import OpenAI from "http://localhost:8080/openai@4.12.4";

Deno.test("issue #743", () => {
  assertEquals(typeof OpenAI, "function");
});
