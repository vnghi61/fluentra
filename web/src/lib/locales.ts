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
export interface TimezoneInfo {
  id: string;
  label: string;
  offset: string;
  searchTerms: string;
}

const TIMEZONE_ALIASES: Record<string, string[]> = {
  "Asia/Ho_Chi_Minh": ["Hanoi", "Ha Noi", "Saigon", "Vietnam", "Việt Nam"],
  "Asia/Bangkok": ["Bangkok", "Thailand"],
  "Asia/Tokyo": ["Tokyo", "Japan"],
  "Asia/Seoul": ["Seoul", "Korea", "South Korea"],
  "Asia/Singapore": ["Singapore"],
  "Asia/Jakarta": ["Jakarta", "Indonesia"],
  "Asia/Manila": ["Manila", "Philippines"],
  "Asia/Kolkata": ["Kolkata", "Calcutta", "Delhi", "Mumbai", "India"],
  "Asia/Dubai": ["Dubai", "UAE", "United Arab Emirates"],
  "Asia/Shanghai": ["Shanghai", "Beijing", "China"],
  "Asia/Hong_Kong": ["Hong Kong"],
  "Asia/Taipei": ["Taipei", "Taiwan"],
  "Europe/London": ["London", "United Kingdom", "UK", "Britain", "England"],
  "Europe/Paris": ["Paris", "France"],
  "Europe/Berlin": ["Berlin", "Germany"],
  "Europe/Rome": ["Rome", "Italy"],
  "Europe/Madrid": ["Madrid", "Spain"],
  "Europe/Moscow": ["Moscow", "Russia"],
  "America/New_York": ["New York", "NYC", "United States", "USA", "US"],
  "America/Chicago": ["Chicago", "United States", "USA", "US"],
  "America/Denver": ["Denver", "United States", "USA", "US"],
  "America/Los_Angeles": [
    "Los Angeles",
    "LA",
    "San Francisco",
    "United States",
    "USA",
    "US",
  ],
  "America/Toronto": ["Toronto", "Canada"],
  "America/Vancouver": ["Vancouver", "Canada"],
  "America/Sao_Paulo": ["Sao Paulo", "Brazil"],
  "Australia/Sydney": ["Sydney", "Australia"],
  "Australia/Melbourne": ["Melbourne", "Australia"],
  "Pacific/Auckland": ["Auckland", "New Zealand"],
  "Africa/Cairo": ["Cairo", "Egypt"],
  "Africa/Johannesburg": ["Johannesburg", "South Africa"],
  UTC: ["UTC", "GMT", "Universal"],
};

export function getTimezoneOffset(timeZone: string): string {
  try {
    const now = new Date();
    const utcDate = new Date(now.toLocaleString("en-US", { timeZone: "UTC" }));
    const tzDate = new Date(now.toLocaleString("en-US", { timeZone }));
    const diffMin = Math.round((tzDate.getTime() - utcDate.getTime()) / 60000);
    const sign = diffMin >= 0 ? "+" : "-";
    const absMin = Math.abs(diffMin);
    const hours = Math.floor(absMin / 60);
    const mins = absMin % 60;
    return mins === 0
      ? `UTC${sign}${hours}`
      : `UTC${sign}${hours}:${mins.toString().padStart(2, "0")}`;
  } catch {
    return "UTC";
  }
}

export function getTimezoneInfo(zone: string): TimezoneInfo {
  const normZone = zone === "Asia/Saigon" ? "Asia/Ho_Chi_Minh" : zone;
  const offset = getTimezoneOffset(normZone);
  const city = normZone.split("/").pop()?.replace(/_/g, " ") ?? normZone;
  const aliases = TIMEZONE_ALIASES[normZone] || [];
  const searchTerms = [
    normZone.toLowerCase(),
    city.toLowerCase(),
    offset.toLowerCase(),
    offset.replace("UTC", "").toLowerCase(),
    ...aliases.map((a) => a.toLowerCase()),
  ].join(" ");

  return {
    id: normZone,
    label: `${normZone.replace(/_/g, " ")} (${offset})`,
    offset,
    searchTerms,
  };
}

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

  // Normalise Asia/Saigon to Asia/Ho_Chi_Minh and ensure Asia/Ho_Chi_Minh is present
  // and deduplicated.
  zones = zones.map((z) => (z === "Asia/Saigon" ? "Asia/Ho_Chi_Minh" : z));
  if (!zones.includes("Asia/Ho_Chi_Minh")) {
    zones.unshift("Asia/Ho_Chi_Minh");
  }
  zones = Array.from(new Set(zones));

  const normCurrent = current === "Asia/Saigon" ? "Asia/Ho_Chi_Minh" : current;
  if (
    normCurrent !== undefined &&
    normCurrent !== "" &&
    !zones.includes(normCurrent)
  ) {
    zones = [normCurrent, ...zones];
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
