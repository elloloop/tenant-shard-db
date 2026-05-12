/*
 * Internal UI wrapper barrel.
 *
 * The rest of the console imports refraction primitives FROM HERE, not
 * from `@refraction-ui/react` directly. This wrapper layer is the
 * project's design-system seam: a single swap point for the underlying
 * library, the only place we apply local defaults, and a curated
 * allow-list of sanctioned components (we wrap only what the app uses
 * today rather than re-exporting all 70+ refraction surfaces).
 *
 * To add a new primitive: create a per-component file in this folder,
 * re-export from there, and add the line below. Do NOT let component
 * code import `@refraction-ui/react` directly.
 */
export * from "./button"
export * from "./dialog"
export * from "./theme-provider"
