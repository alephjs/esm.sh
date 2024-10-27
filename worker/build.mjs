import { readFileSync, writeFileSync } from "node:fs";
import { build as esbuild } from "esbuild";

const build = (options) => {
  return esbuild({
    target: "esnext",
    format: "esm",
    platform: "browser",
    outdir: "dist",
    bundle: true,
    minify: true,
    logLevel: "info",
    ...options,
  });
};
const toJS = (s) => JSON.parse(s.split("\n").map((line) => line.split("//")[0].trim()).join("\n").replace(/,\n}/g, "\n}"));

const goCode = readFileSync("../server/consts.go", "utf8");
const [, version] = goCode.match(/VERSION = (\d+)/);
const [, cssPackages] = goCode.match(/cssPackages = map\[string\]string(\{[\s\S]+?\})/);
const [, assetExts] = goCode.match(/assetExts = map\[string\]bool(\{[\s\S]+?\})/);
const constsTs = `// generated by \`build.mjs\`, do not edit manually.
export const VERSION = ${version};
export const assetsExts = new Set(${JSON.stringify(Object.keys(toJS(assetExts)))});
export const cssPackages = ${JSON.stringify(toJS(cssPackages))};
`;

await writeFileSync("./src/consts.ts", constsTs);
await build({ entryPoints: ["src/index.ts"] });
