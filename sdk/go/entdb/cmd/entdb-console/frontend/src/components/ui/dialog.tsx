/*
 * Dialog wrapper.
 *
 * App code imports these names from `@/components/ui`, never directly
 * from `@refraction-ui/react`. That gives us one place to apply local
 * defaults (or swap the implementation) without touching call sites.
 *
 * The refraction Dialog is a compound component — Dialog (provider) +
 * DialogContent + DialogOverlay + DialogHeader/Title/Description/Footer
 * + DialogClose. We re-export the pieces the console actually uses.
 */
export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  DialogTrigger,
} from "@refraction-ui/react"
