import { assertEquals } from "jsr:@std/assert";

import { customElement } from "http://localhost:8080/lit@2.7.2/decorators";
import { map } from "http://localhost:8080/lit@2.7.2/directives/map";

Deno.test("issue #599", async () => {
  assertEquals(typeof customElement, "function");
  assertEquals(typeof map, "function");
});
