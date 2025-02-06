import { assert, assertEquals, assertStringIncludes } from "jsr:@std/assert";

// change the import path to the module you want to test
import * as mod from "esm.sh/PKG[@SEMVER][/PATH]";

// related issue: https://github.com/esm-dev/esm.sh/issues/ISSUE_NUMBER
Deno.test("testing name", async () => {
  assert("foo" in mod);
  assertEquals(typeof mod.foo, "function");
  assertStringIncludes(mod.foo(), "bar");
});
