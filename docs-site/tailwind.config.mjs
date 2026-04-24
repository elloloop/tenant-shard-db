import { refractionPreset } from "@refraction-ui/tailwind-config";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}"],
  presets: [refractionPreset],
  darkMode: "class",
  // The .dark class is toggled by the theme script at runtime, and Tailwind's
  // JIT only emits a base-layer rule if its selector appears in `content`.
  // Safelist `dark` so the CSS-variable overrides survive the purge step.
  safelist: ["dark"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["var(--font-sans)"],
        mono: ["var(--font-mono)"],
      },
    },
  },
};
