import { inject, type InjectionKey } from "vue";
import type { useAdminController } from "../../composables/admin/useAdminController";

export type AdminViewContext = ReturnType<typeof useAdminController>;

export const adminViewContextKey: InjectionKey<AdminViewContext> =
  Symbol("admin-view-context");

export function useAdminViewContext(): AdminViewContext {
  const context = inject(adminViewContextKey);
  if (!context) throw new Error("Admin view context is missing");
  return context;
}
