import { assertEquals } from "jsr:@std/assert";

import styleToJS from "http://localhost:8080/style-to-js@1.1.3";

Deno.test("issue #583", async () => {
  assertEquals(styleToJS("width:100%;", { reactCompat: true }), {
    width: "100%",
  });
});
