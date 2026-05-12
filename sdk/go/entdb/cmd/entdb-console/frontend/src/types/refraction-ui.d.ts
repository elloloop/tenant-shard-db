/*
 * Ambient type shim for `@refraction-ui/react` 0.4.1.
 *
 * The package's published `dist/index.d.ts` re-exports from individual
 * sub-package paths (e.g. `@refraction-ui/react-dialog`) that aren't
 * shipped — the runtime bundle inlines them, but the type graph
 * dangles. Until upstream republishes consolidated `.d.ts` output,
 * we declare the handful of primitives the console actually uses here.
 *
 * This shim only types the wrapper-layer surface in `src/components/ui/`;
 * the wrapper layer in turn is the only thing the rest of the app ever
 * sees of refraction-ui. When upstream fixes the types we can delete
 * this file in one PR with no other code changes.
 */
declare module "@refraction-ui/react" {
  import type {
    ButtonHTMLAttributes,
    ForwardRefExoticComponent,
    HTMLAttributes,
    ReactNode,
    RefAttributes,
  } from "react"

  // ── Button ───────────────────────────────────────────────────────────
  export type ButtonVariant =
    | "default"
    | "destructive"
    | "outline"
    | "secondary"
    | "ghost"
    | "link"
  export type ButtonSize = "xs" | "sm" | "default" | "lg" | "icon"

  export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: ButtonVariant
    size?: ButtonSize
    asChild?: boolean
  }

  export const Button: ForwardRefExoticComponent<
    ButtonProps & RefAttributes<HTMLButtonElement>
  >
  export const buttonVariants: (props?: {
    variant?: ButtonVariant
    size?: ButtonSize
  }) => string

  // ── Dialog (compound) ────────────────────────────────────────────────
  export interface DialogProps {
    open?: boolean
    defaultOpen?: boolean
    onOpenChange?: (open: boolean) => void
    modal?: boolean
    children?: ReactNode
  }
  export const Dialog: (props: DialogProps) => JSX.Element

  export const DialogTrigger: ForwardRefExoticComponent<
    ButtonHTMLAttributes<HTMLButtonElement> & RefAttributes<HTMLButtonElement>
  >
  export const DialogOverlay: ForwardRefExoticComponent<
    HTMLAttributes<HTMLDivElement> & RefAttributes<HTMLDivElement>
  >
  export const DialogContent: ForwardRefExoticComponent<
    HTMLAttributes<HTMLDivElement> & RefAttributes<HTMLDivElement>
  >
  export const DialogHeader: ForwardRefExoticComponent<
    HTMLAttributes<HTMLDivElement> & RefAttributes<HTMLDivElement>
  >
  export const DialogFooter: ForwardRefExoticComponent<
    HTMLAttributes<HTMLDivElement> & RefAttributes<HTMLDivElement>
  >
  export const DialogTitle: ForwardRefExoticComponent<
    HTMLAttributes<HTMLHeadingElement> & RefAttributes<HTMLHeadingElement>
  >
  export const DialogDescription: ForwardRefExoticComponent<
    HTMLAttributes<HTMLParagraphElement> & RefAttributes<HTMLParagraphElement>
  >
  export const DialogClose: ForwardRefExoticComponent<
    ButtonHTMLAttributes<HTMLButtonElement> & RefAttributes<HTMLButtonElement>
  >
}

declare module "@refraction-ui/react/theme" {
  import type { ReactNode } from "react"

  export type ThemeMode = "light" | "dark" | "system"
  export type ResolvedMode = "light" | "dark"

  export interface ThemeProviderProps {
    children: ReactNode
    defaultMode?: ThemeMode
    storageKey?: string
    attribute?: "class" | "data-theme"
  }
  export const ThemeProvider: (props: ThemeProviderProps) => JSX.Element

  export interface ThemeToggleProps {
    className?: string
    variant?: "segmented" | "icon" | "switch"
  }
  export const ThemeToggle: (props: ThemeToggleProps) => JSX.Element

  export function useTheme(): {
    mode: ThemeMode
    resolved: ResolvedMode
    setMode: (mode: ThemeMode) => void
  }
}
