/**
 * Country and timezone option lists for the profile form.
 *
 * Both fields used to be free text. A learner typed their country and their
 * timezone by hand into inputs whose only validation was "two characters" and
 * "not empty" — so `Viet Nam` was rejected for being three letters and `VN` was
 * accepted alongside `XX`, and a timezone the server could not parse was stored
 * and silently made every scheduled review land on the wrong local day.
 *
 * The names come from `Intl`, not from a bundled table: the browser already
 * ships them, they follow the reader's own locale, and a table we maintain would
 * be out of date the first time a country renamed itself.
 */

/**
 * ISO 3166-1 alpha-2 codes. The server's own pattern is `^[A-Z]{2}$`, so the
 * value stored is unchanged — this list only decides what can be picked.
 */
const COUNTRY_CODES = [
  "AD",
  "AE",
  "AF",
  "AG",
  "AL",
  "AM",
  "AO",
  "AR",
  "AT",
  "AU",
  "AZ",
  "BA",
  "BB",
  "BD",
  "BE",
  "BF",
  "BG",
  "BH",
  "BI",
  "BJ",
  "BN",
  "BO",
  "BR",
  "BS",
  "BT",
  "BW",
  "BY",
  "BZ",
  "CA",
  "CD",
  "CF",
  "CG",
  "CH",
  "CI",
  "CL",
  "CM",
  "CN",
  "CO",
  "CR",
  "CU",
  "CV",
  "CY",
  "CZ",
  "DE",
  "DJ",
  "DK",
  "DM",
  "DO",
  "DZ",
  "EC",
  "EE",
  "EG",
  "ER",
  "ES",
  "ET",
  "FI",
  "FJ",
  "FM",
  "FR",
  "GA",
  "GB",
  "GD",
  "GE",
  "GH",
  "GM",
  "GN",
  "GQ",
  "GR",
  "GT",
  "GW",
  "GY",
  "HN",
  "HR",
  "HT",
  "HU",
  "ID",
  "IE",
  "IL",
  "IN",
  "IQ",
  "IR",
  "IS",
  "IT",
  "JM",
  "JO",
  "JP",
  "KE",
  "KG",
  "KH",
  "KI",
  "KM",
  "KN",
  "KP",
  "KR",
  "KW",
  "KZ",
  "LA",
  "LB",
  "LC",
  "LI",
  "LK",
  "LR",
  "LS",
  "LT",
  "LU",
  "LV",
  "LY",
  "MA",
  "MC",
  "MD",
  "ME",
  "MG",
  "MH",
  "MK",
  "ML",
  "MM",
  "MN",
  "MR",
  "MT",
  "MU",
  "MV",
  "MW",
  "MX",
  "MY",
  "MZ",
  "NA",
  "NE",
  "NG",
  "NI",
  "NL",
  "NO",
  "NP",
  "NR",
  "NZ",
  "OM",
  "PA",
  "PE",
  "PG",
  "PH",
  "PK",
  "PL",
  "PT",
  "PW",
  "PY",
  "QA",
  "RO",
  "RS",
  "RU",
  "RW",
  "SA",
  "SB",
  "SC",
  "SD",
  "SE",
  "SG",
  "SI",
  "SK",
  "SL",
  "SM",
  "SN",
  "SO",
  "SR",
  "SS",
  "ST",
  "SV",
  "SY",
  "SZ",
  "TD",
  "TG",
  "TH",
  "TJ",
  "TL",
  "TM",
  "TN",
  "TO",
  "TR",
  "TT",
  "TV",
  "TW",
  "TZ",
  "UA",
  "UG",
  "US",
  "UY",
  "UZ",
  "VA",
  "VC",
  "VE",
  "VN",
  "VU",
  "WS",
  "YE",
  "ZA",
  "ZM",
  "ZW",
] as const;

export interface CountryOption {
  code: string;
  name: string;
}

/**
 * Countries named in `locale`, sorted by that locale's own collation.
 *
 * Sorted with `Intl.Collator` rather than by code point, because Vietnamese
 * names put Đ between D and E and a plain sort does not.
 */
export function countryOptions(locale: string): CountryOption[] {
  let naming: Intl.DisplayNames | null = null;
  try {
    naming = new Intl.DisplayNames([locale], { type: "region" });
  } catch {
    // An unsupported locale is not a reason to lose the list; the codes still
    // identify the country, and the server only ever stores the code.
    naming = null;
  }

  const collator = new Intl.Collator(locale);
  return COUNTRY_CODES.map((code) => ({
    code,
    name: naming?.of(code) ?? code,
  })).sort((a, b) => collator.compare(a.name, b.name));
}

/**
 * The IANA zones this browser knows, or a short list covering the learners this
 * product has.
 *
 * `Intl.supportedValuesOf` is the authority when it exists: it is the same set
 * the browser will accept back, so a learner cannot pick a zone their own
 * device cannot resolve. The fallback is not a substitute for it — it is what
 * keeps the field usable on a browser too old to enumerate, and it always
 * carries the learner's current zone so their own setting is never absent from
 * the list of things they may choose.
 */
export function timezoneOptions(current?: string): string[] {
  let zones: string[] = [];
  try {
    const supported = (
      Intl as unknown as { supportedValuesOf?: (key: string) => string[] }
    ).supportedValuesOf;
    if (typeof supported === "function") {
      zones = supported("timeZone");
    }
  } catch {
    zones = [];
  }

  if (zones.length === 0) {
    zones = [
      "Asia/Ho_Chi_Minh",
      "Asia/Bangkok",
      "Asia/Singapore",
      "Asia/Tokyo",
      "Asia/Seoul",
      "Asia/Shanghai",
      "Asia/Kolkata",
      "Australia/Sydney",
      "Europe/London",
      "Europe/Paris",
      "Europe/Berlin",
      "Europe/Moscow",
      "America/New_York",
      "America/Chicago",
      "America/Denver",
      "America/Los_Angeles",
      "America/Sao_Paulo",
      "Africa/Cairo",
      "Africa/Lagos",
      "UTC",
    ];
  }

  if (current !== undefined && current !== "" && !zones.includes(current)) {
    zones = [current, ...zones];
  }
  return zones;
}

/**
 * The daily study goals the form offers, in minutes.
 *
 * A number input accepted 7, 113 and 0. The server's own bounds are 5 to 180,
 * and a goal is a commitment a learner keeps or misses — the value of picking
 * 113 over 120 is nil, and the cost of typing 1130 by accident is a streak that
 * can never be kept.
 */
export const DAILY_GOAL_MINUTES = [5, 10, 15, 20, 30, 45, 60, 90, 120, 180];
