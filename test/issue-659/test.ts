import { assertEquals } from "jsr:@std/assert";

import { createNodeMiddleware } from "http://localhost:8080/@octokit/oauth-app@4.2.2";

Deno.test("issue #659", () => {
  assertEquals(typeof createNodeMiddleware, "function");
});
