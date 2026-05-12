/*
 * Button wrapper.
 *
 * App code imports `Button` from `@/components/ui`, never directly from
 * `@refraction-ui/react`. The refraction button already exposes
 * `variant` (default | destructive | outline | secondary | ghost | link)
 * and `size` (xs | sm | default | lg | icon), which covers everything
 * the console needs today.
 */
export { Button, buttonVariants } from "@refraction-ui/react"
