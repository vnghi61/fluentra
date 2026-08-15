import { useTranslation } from "react-i18next";

/** Placeholder route. It exists so the router has something to route *to*. */
export function PracticePage(): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <div className="space-y-2">
      <h1 className="text-2xl font-bold">{t("nav.practice")}</h1>
      <p className="text-sm text-slate-400">{t("app.tagline")}</p>
    </div>
  );
}
