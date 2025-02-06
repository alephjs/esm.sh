import { TextLineStream } from "jsr:@std/streams@1.0.7/text-line-stream";

const enc = new TextEncoder();
const regexpVuePath = /^\/\*?vue@([~\^]?[\w\+\-\.]+)(\/|\?|&|$)/;
const regexpSveltePath = /^\/\*?svelte@([~\^]?[\w\+\-\.]+)(\/|\?|&|$)/;
const output = (type, data) => Deno.stdout.write(enc.encode(">>>" + type + ":" + JSON.stringify(data) + "\n"));

let tsx;
let unoGenerators;

async function transformModule(filename, importMap, sourceCode) {
  const imports = importMap?.imports;
  if (imports) {
    for (const [specifier, resolved] of Object.entries(imports)) {
      if (
        (specifier === "react-dom" || specifier === "react-dom/client" || specifier === "vue")
        && (resolved.startsWith("https://") || resolved.startsWith("http://"))
      ) {
        const url = new URL(resolved);
        const query = url.searchParams;
        if (!query.has("dev")) {
          query.set("dev", "true");
          imports[specifier] = url.origin + url.pathname + url.search.replace("dev=true", "dev");
        }
      }
    }
  }
  let lang = filename.endsWith(".md?jsx") ? "jsx" : undefined;
  let code = sourceCode ?? await Deno.readTextFile("." + filename);
  let preprocessSM = undefined;
  if (filename.endsWith(".svelte") || filename.endsWith(".md?svelte")) {
    [lang, code, preprocessSM] = await transformSvelte(filename, code, importMap, true);
  } else if (filename.endsWith(".vue") || filename.endsWith(".md?vue")) {
    [lang, code, preprocessSM] = await transformVue(filename, code, importMap, true);
  }
  if (!tsx) {
    tsx = import("npm:@esm.sh/tsx@1.0.5").then(async ({ init, transform }) => {
      await init();
      return { transform };
    });
  }
  const { transform } = await tsx;
  const ret = transform({
    filename,
    lang,
    code,
    importMap,
    sourceMap: preprocessSM ? "external" : "inline",
    dev: {
      hmr: { runtime: "/@hmr" },
      refresh: imports?.react && !imports?.preact ? { runtime: "/@refresh" } : undefined,
      prefresh: imports?.preact ? { runtime: "/@prefresh" } : undefined,
    },
  });
  let js = ret.code;
  if (ret.map) {
    if (preprocessSM) {
      // todo: merge preprocess source map
    }
    js += "\n//# sourceMappingURL=data:application/json;base64," + btoa(ret.map);
  }
  return js;
}

async function transformVue(filename, sourceCode, importMap, isDev) {
  const { transform } = await import("npm:@esm.sh/vue-compiler@1.0.1");
  const ret = await transform(filename, sourceCode, {
    imports: { "@vue/compiler-sfc": import("npm:@vue/compiler-sfc@" + getVueVersion(importMap)) },
    isDev,
    devRuntime: isDev ? "/@vdr" : undefined,
  });
  return [ret.lang, ret.code, ret.map];
}

async function transformSvelte(filename, sourceCode, importMap, isDev) {
  const { compile, VERSION } = await import(`npm:svelte@${getSvelteVersion(importMap)}/compiler`);
  const majorVersion = parseInt(VERSION.split(".", 1)[0]);
  if (majorVersion < 5) {
    throw new Error("Unsupported Svelte version: " + VERSION + ". Please use svelte@5 or higher.");
  }
  const { js } = compile(sourceCode, {
    filename,
    css: "injected",
    dev: isDev,
    hmr: isDev,
  });
  return ["js", js.code, js.map];
}

function getVueVersion(importMap) {
  const vueUrl = importMap?.imports?.vue;
  if (vueUrl && isHttpSpecifier(vueUrl)) {
    const url = new URL(vueUrl);
    const m = url.pathname.match(regexpVuePath);
    if (m) {
      return m[1];
    }
  }
  // default to vue@3
  return "3";
}

function getSvelteVersion(importMap) {
  const svelteUrl = importMap?.imports?.svelte;
  if (svelteUrl && isHttpSpecifier(svelteUrl)) {
    const url = new URL(svelteUrl);
    const m = url.pathname.match(regexpSveltePath);
    if (m) {
      return m[1];
    }
  }
  // default to svelte@5
  return "5";
}

function isHttpSpecifier(specifier) {
  return typeof specifier === "string" && specifier.startsWith("https://") || specifier.startsWith("http://");
}

async function unocss(config, content, id) {
  const generatorId = config?.filename ?? ".";
  if (!unoGenerators) {
    unoGenerators = new Map();
  }
  let uno = unoGenerators.get(generatorId);
  if (!uno || uno.configCSS !== config?.css) {
    uno = import("npm:@esm.sh/unocss@0.4.3").then(({ init }) => init({ configCSS: config?.css }));
    uno.configCSS = config?.css;
    unoGenerators.set(generatorId, uno);
  }
  const { update, generate } = await uno;
  await update(content, id);
  return await generate();
}

for await (const line of Deno.stdin.readable.pipeThrough(new TextDecoderStream()).pipeThrough(new TextLineStream())) {
  try {
    const [type, ...args] = JSON.parse(line);
    switch (type) {
      case "unocss":
        output("css", await unocss(...args));
        break;
      case "module":
        output("js", await transformModule(...args));
        break;
      case "vue": {
        const [lang, code] = await transformVue(...args);
        output(lang, code);
        break;
      }
      case "svelte": {
        const [lang, code] = await transformSvelte(...args);
        output(lang, code);
        break;
      }
      default:
        output("error", "Unknown loader type: " + type);
    }
  } catch (e) {
    output("error", e.message);
  }
}
