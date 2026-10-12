/**
 * Answer feedback and explanations render in the interface language only,
 * falling back to the other language when the chosen one is empty.
 */

export function prefersVietnamese(
  language: string | null | undefined,
): boolean {
  if (!language) {
    return false;
  }
  return language.toLowerCase().startsWith("vi");
}

export function pickLocalized(
  en: string | null | undefined,
  vi: string | null | undefined,
  language: string | null | undefined,
): string {
  const trimmedEn = en?.trim() ?? "";
  const trimmedVi = vi?.trim() ?? "";

  const chosen = prefersVietnamese(language) ? trimmedVi : trimmedEn;
  const fallback = prefersVietnamese(language) ? trimmedEn : trimmedVi;

  return chosen.length > 0 ? chosen : fallback;
}
