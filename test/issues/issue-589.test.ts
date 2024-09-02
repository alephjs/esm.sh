import { assert, assertEquals } from "jsr:@std/assert";

Deno.test("issue #589", async () => {
  const res = await fetch(
    "http://localhost:8080/@types/react@^18/index.d.ts",
    { redirect: "manual" },
  );
  res.body?.cancel();

  assertEquals(res.status, 302);
  assert(
    res.headers.get("location")!.startsWith(
      `http://localhost:8080/@types/react@18.`,
    ),
  );
});
