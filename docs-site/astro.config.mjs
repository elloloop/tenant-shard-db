import { defineConfig } from "astro/config";
import tailwind from "@astrojs/tailwind";

export default defineConfig({
  site: "https://elloloop.github.io",
  base: "/tenant-shard-db",
  integrations: [tailwind({ applyBaseStyles: false })],
  output: "static",
  markdown: {
    shikiConfig: {
      theme: "github-dark-dimmed",
      wrap: false,
    },
  },
});
