import { spawn } from "node:child_process";
import { readdirSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const port = 41739;
const origin = `http://127.0.0.1:${port}`;
const vite = resolve(root, "node_modules/vite/bin/vite.js");
const server = spawn(
  process.execPath,
  [
    vite,
    "preview",
    "--host",
    "127.0.0.1",
    "--port",
    String(port),
    "--strictPort",
  ],
  { cwd: root, stdio: ["ignore", "pipe", "pipe"] },
);

let diagnostics = "";
server.stdout.on("data", (chunk) => (diagnostics += chunk));
server.stderr.on("data", (chunk) => (diagnostics += chunk));

async function request(path) {
  const response = await fetch(`${origin}${path}`, { cache: "no-store" });
  if (!response.ok) throw new Error(`${path}: HTTP ${response.status}`);
  const body = await response.arrayBuffer();
  return {
    path,
    status: response.status,
    bytes: body.byteLength,
    text: new TextDecoder().decode(body),
  };
}

async function waitUntilReady() {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    try {
      return await request("/index.html");
    } catch {
      await new Promise((resolveDelay) => setTimeout(resolveDelay, 250));
    }
  }
  throw new Error(`preview startup timeout\n${diagnostics}`);
}

try {
  const index = await waitUntilReady();
  const results = [
    index,
    await request("/home.html"),
    await request("/user.html"),
  ];
  const entry = index.text.match(/src="([^\"]+index-[^\"]+\.js)"/)?.[1];
  if (!entry) throw new Error("admin entry asset not found");
  if (/modulepreload[^>]+editor-vendor/.test(index.text)) {
    throw new Error("editor-vendor must not be preloaded by the admin entry");
  }
  results.push(await request(entry));

  const chunks = readdirSync(resolve(root, "../core/admin/assets"))
    .filter((name) => /View-.+\.js$/.test(name))
    .sort();
  if (chunks.length !== 11)
    throw new Error(`expected 11 lazy view chunks, got ${chunks.length}`);
  for (const chunk of chunks) results.push(await request(`/assets/${chunk}`));
  const editorChunk = readdirSync(resolve(root, "../core/admin/assets")).find(
    (name) => /^editor-vendor-.+\.js$/.test(name),
  );
  if (!editorChunk) throw new Error("lazy editor chunk not found");
  const editorResult = await request(`/assets/${editorChunk}`);

  console.log(
    JSON.stringify(
      {
        pages: results
          .slice(0, 3)
          .map(({ path, status, bytes }) => ({ path, status, bytes })),
        entry: results
          .slice(3, 4)
          .map(({ path, status, bytes }) => ({ path, status, bytes }))[0],
        lazy_view_chunks: chunks.length,
        lazy_view_bytes: results
          .slice(4)
          .reduce((total, item) => total + item.bytes, 0),
        editor_lazy_chunk: {
          path: editorResult.path,
          status: editorResult.status,
          bytes: editorResult.bytes,
          preloaded: false,
        },
        all_status_200: results.every((item) => item.status === 200),
      },
      null,
      2,
    ),
  );
} finally {
  server.kill();
}
