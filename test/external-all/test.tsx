import { assert } from "https://deno.land/std@0.220.0/assert/mod.ts";

import { h } from "preact";
import render from "preact-render-to-string";
import useSWR from "swr";

Deno.test("external all", () => {
  const fetcher = (url: string) => fetch(url).then((res) => res.json());
  const App = () => {
    const { data } = useSWR("http://localhost:8080/status.json", fetcher, {
      fallbackData: { uptime: "just now" },
    });
    if (!data) {
      return (
        <main>
          <p>loading...</p>
        </main>
      );
    }
    return (
      <main>
        <p>{data.uptime}</p>
      </main>
    );
  };
  const html = render(<App />);
  assert(html == "<main><p>just now</p></main>");
});
