import { assertEquals } from "jsr:@std/assert";

import * as nfn from "http://localhost:8080/node-fetch-native";

Deno.test("issue #422", () => {
  // @ts-ignore
  assertEquals(nfn.fileFrom, undefined);
});
