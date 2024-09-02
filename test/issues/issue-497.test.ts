import { assertEquals } from "jsr:@std/assert";

import wretch from "http://localhost:8080/wretch@2.4.1";

Deno.test("issue #497", async () => {
  let status: Record<string, unknown> = {};
  await new Promise<void>((resolve) => {
    wretch("http://localhost:8080/status.json").get().json((d: any) => {
      status = d;
      resolve();
    });
  });
  assertEquals(typeof status.version, "number");
});
