import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import enUS from "./locales/en-US.json";

// This fork supports English only, including fallback from legacy saved locales.
export const LANGS = [
  { code: "en-US", label: "English" },
] as const;

export type LangCode = (typeof LANGS)[number]["code"];

export const DEFAULT_LANG: LangCode = "en-US";
export const LANG_STORAGE_KEY = "antifox-lang";

export function isLangCode(v: string | null | undefined): v is LangCode {
  return !!v && LANGS.some((l) => l.code === v);
}

// Bundle resources synchronously for static exports and consistent SSR/initial rendering.
if (!i18n.isInitialized) {
  void i18n.use(initReactI18next).init({
    resources: {
      "en-US": { translation: enUS },
    },
    lng: DEFAULT_LANG,
    fallbackLng: DEFAULT_LANG,
    interpolation: { escapeValue: false },
    returnNull: false,
  });
}

export default i18n;
