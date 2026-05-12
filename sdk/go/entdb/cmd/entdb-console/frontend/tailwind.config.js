import { refractionPreset } from "@refraction-ui/tailwind-config"

/** @type {import('tailwindcss').Config} */
export default {
  // The refraction preset already sets `darkMode: "class"` and declares
  // colors / radii / fonts / animations bound to the same `--background`,
  // `--foreground`, `--card` (etc.) HSL variables our existing component
  // tree consumes — so the preset is a drop-in replacement for the
  // hand-rolled token mapping that used to live here.
  presets: [refractionPreset],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
    // refraction-ui's React components ship Tailwind utility classes inside
    // node_modules. Without scanning them, classes like dialog overlays'
    // `bg-black/80` and `z-50` get purged and the modal backdrop becomes
    // invisible. Matches the equivalent scan path docs-site uses for the
    // Astro variant.
    "./node_modules/@refraction-ui/react/dist/**/*.{js,mjs,cjs}",
  ],
  // The .dark class is toggled by ThemeProvider at runtime; Tailwind's JIT
  // only emits the dark-mode rules if it sees `dark` somewhere in content.
  // Safelist it so the CSS-variable overrides survive purge.
  safelist: ["dark"],
}
