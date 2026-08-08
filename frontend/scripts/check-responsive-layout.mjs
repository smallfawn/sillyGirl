import { readFile } from "node:fs/promises";

const read = (path) => readFile(new URL(`../${path}`, import.meta.url), "utf8");

const entries = await Promise.all(
  ["index.html", "home.html", "user.html"].map(async (path) => ({
    path,
    content: await read(path),
  })),
);
const styles = await read("src/styles.css");
const home = await read("src/Home.vue");
const user = await read("src/User.vue");

const checks = {
  entryViewportFit: entries.every(({ content }) =>
    content.includes("viewport-fit=cover"),
  ),
  entryKeyboardResize: entries.every(({ content }) =>
    content.includes("interactive-widget=resizes-content"),
  ),
  safeAreaInsets: styles.includes("env(safe-area-inset-top"),
  dynamicViewport: [styles, home, user].every((content) =>
    content.includes("100dvh"),
  ),
  touchPointerRules:
    styles.includes("(hover: none) and (pointer: coarse)") &&
    styles.includes("min-height: 44px"),
  phoneBreakpoints:
    styles.includes("@media (max-width: 600px)") &&
    styles.includes("@media (max-width: 480px)"),
  tabletGrid: styles.includes(
    "repeat(auto-fit, minmax(min(280px, 100%), 1fr))",
  ),
  tableScrolling:
    styles.includes("-webkit-overflow-scrolling: touch") &&
    styles.includes("overscroll-behavior-inline: contain"),
  platformFonts:
    styles.includes('"SF Pro Text"') &&
    styles.includes('"Segoe UI"') &&
    styles.includes("Roboto") &&
    styles.includes('"HarmonyOS Sans SC"'),
};

const failed = Object.entries(checks)
  .filter(([, pass]) => !pass)
  .map(([name]) => name);

console.log(JSON.stringify({ checks, failed }, null, 2));
if (failed.length) process.exit(1);
