import { assertEquals } from "jsr:@std/assert";

import ws4 from "http://localhost:8080/isomorphic-ws@4";
import ws5 from "http://localhost:8080/isomorphic-ws@5";

Deno.test("issue #381 (isomorphic-ws)", () => {
  assertEquals(typeof ws4, "function");
  assertEquals(typeof ws5, "function");
});
