import { defineConfig } from "astro/config";
import react from "@astrojs/react";

export default defineConfig({
  site: "https://elloloop.github.io",
  base: "/tenant-shard-db",
  integrations: [react()],
  output: "static",
});
