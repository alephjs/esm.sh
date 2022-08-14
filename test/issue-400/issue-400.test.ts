import { equal } from "https://deno.land/std@0.145.0/testing/asserts.ts";

import chalk from "http://localhost:8080/chalk@5.0.1";

Deno.test("issue #400", () => {
  equal(chalk.blue.bgRed.bold("Hello world!"), "Hello world!");
});
