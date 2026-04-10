import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwind from "@astrojs/tailwind";

export default defineConfig({
  site: "https://elloloop.github.io",
  base: "/tenant-shard-db",
  integrations: [react(), tailwind()],
  output: "static",
});
