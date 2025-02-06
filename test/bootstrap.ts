#!/usr/bin/env -S deno run --allow-run --allow-read --allow-write --allow-net

async function startServer(onStart: () => Promise<void>) {
  const { code, success } = await run("go", "build", "-tags", "debug", "-o", "esmd", "server/cmd/main.go");
  if (!success) {
    Deno.exit(code);
  }
  let configJson = {};
  try {
    configJson = JSON.parse(await Deno.readTextFile("config.json"));
  } catch {
    // ignore
  }
  await Deno.writeTextFile(
    "config.json",
    JSON.stringify(
      {
        "port": 8080,
        "workDir": ".esmd",
        "legacyServer": "https://legacy.esm.sh",
        ...configJson,
      },
      undefined,
      2,
    ),
  );
  const p = new Deno.Command("./esmd", {
    stdout: Deno.args.includes("-q") ? "null" : "inherit",
    stderr: "inherit",
  }).spawn();
  addEventListener("unload", () => {
    console.log("%cClosing esm.sh server...", "color: grey");
    p.kill("SIGINT");
  });
  await new Promise<void>((resolve, reject) => {
    (async () => {
      while (true) {
        try {
          await new Promise((resolve) => setTimeout(resolve, 100));
          const status = await fetch("http://localhost:8080/status.json").then(res => res.json());
          if (status.version) {
            console.log("esm.sh server started.");
            onStart();
            resolve();
            break;
          }
        } catch {
          // continue
        }
      }
    })();
    setTimeout(() => reject(new Error("Timeout")), 15000);
  });
  await p.status;
}

async function runTest(name: string, retry?: boolean): Promise<number> {
  const execBegin = Date.now();
  const args = [
    "test",
    "-A",
    "--unstable-fs",
    "--check",
    "--no-lock",
    "--reload=http://localhost:8080",
    "--location=http://0.0.0.0/",
    "-q",
  ].filter(Boolean);
  const dir = `test/${name}/`;
  if (await exists(dir + "deno.json")) {
    args.push("--config", dir + "deno.json");
  }
  args.push(dir);
  console.log(`\n[test ${name}]`);
  const { code, success } = await run(Deno.execPath(), ...args);
  if (!success) {
    if (!retry) {
      console.log("something wrong, retry...");
      await new Promise((resolve) => setTimeout(resolve, 500));
      return await runTest(name, true);
    } else {
      Deno.exit(code);
    }
  }
  return Date.now() - execBegin;
}

function run(name: string, ...args: string[]) {
  const p = new Deno.Command(name, {
    args,
    stdout: "inherit",
    stderr: "inherit",
  }).spawn();
  return p.status;
}

async function exists(path: string): Promise<boolean> {
  try {
    await Deno.lstat(path);
    return true;
  } catch (err) {
    if (err instanceof Deno.errors.NotFound) {
      return false;
    }
    throw err;
  }
}

if (import.meta.main) {
  Deno.chdir(new URL("../", import.meta.url).pathname);
  const tests = Deno.args.filter((arg) => !arg.startsWith("-"));
  for (const testDir of tests) {
    if (!(await exists(`test/${testDir}`))) {
      console.error(`Test directory "${testDir}" not found.`);
      Deno.exit(1);
    }
  }
  try {
    console.log("Cleaning up...");
    await Promise.all([
      Deno.remove(".esmd/esm.db"),
      Deno.remove(".esmd/storage", { recursive: true }),
      Deno.remove(".esmd/log", { recursive: true }),
    ]);
  } catch (_) {
    // ignore
  }
  console.log("Starting esm.sh server...");
  startServer(async () => {
    let timeUsed = 0;
    if (tests.length > 0) {
      for (const testDir of tests) {
        timeUsed += await runTest(testDir, true);
      }
    } else {
      const dirs: string[] = [];
      for await (const entry of Deno.readDir("./test")) {
        if (entry.isDirectory && !entry.name.startsWith("_") && !entry.name.startsWith(".")) {
          dirs.push(entry.name);
        }
      }
      for (const dir of dirs.sort()) {
        timeUsed += await runTest(dir);
      }
    }
    timeUsed = Math.ceil(timeUsed / 1000);
    console.log(
      `Done! Total time spent: %c${Math.floor(timeUsed / 60)}m${timeUsed % 60}s`,
      "color: blue",
    );
    Deno.exit(0);
  });
}
