/*
 * ThemeProvider wrapper.
 *
 * Refraction ships its theme primitives behind the `/theme` subpath
 * export so SSR/CSR bundles can opt in without dragging the whole
 * component graph. We re-export from here so the rest of the app stays
 * blind to that detail (and to the `@refraction-ui/react` package name
 * in general — the wrapper layer is the only place that knows).
 */
export { ThemeProvider, ThemeToggle, useTheme } from "@refraction-ui/react/theme"
