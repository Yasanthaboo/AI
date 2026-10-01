import type { ServerRates } from "./api";

export type Settings = {
  rates: ServerRates;
  currency: string;
  exchangeRate: number;
  preferredUnit: string;
  defaultWallHeight: { value: number; unit: string };
};

export const settingsKey = "quantity-surveyor-settings";
export const units = ["m", "cm", "ft", "in"];
export const currencies = ["USD", "EUR", "GBP", "AUD", "CAD", "INR", "LKR", "AED"];
const metersPerUnit: Record<string, number> = { m: 1, cm: 0.01, ft: 0.3048, in: 0.0254 };

export const defaultSettings: Settings = {
  rates: { plasterUsdPerSquareMeter: 10, paintUsdPerSquareMeter: 5, flooringUsdPerSquareMeter: 0, ceilingUsdPerSquareMeter: 0, skirtingUsdPerMeter: 0, tilingUsdPerSquareMeter: 0, wastePercent: 0 },
  currency: "USD",
  exchangeRate: 1,
  preferredUnit: "m",
  defaultWallHeight: { value: 6, unit: "ft" },
};

const finite = (value: unknown, fallback: number, min = 0, max = Infinity) => typeof value === "number" && Number.isFinite(value) && value >= min && value <= max ? value : fallback;

// Browser storage is untrusted input: keep only known keys with valid values.
export function loadSettings(): Settings {
  try {
    const saved = JSON.parse(localStorage.getItem(settingsKey) || "null") as Partial<Settings> | null;
    if (!saved || typeof saved !== "object") return defaultSettings;
    const rates = { ...defaultSettings.rates };
    for (const key of Object.keys(rates) as (keyof ServerRates)[]) rates[key] = finite(saved.rates?.[key], rates[key], 0, key === "wastePercent" ? 50 : Infinity);
    if (!(rates.plasterUsdPerSquareMeter > 0)) rates.plasterUsdPerSquareMeter = defaultSettings.rates.plasterUsdPerSquareMeter;
    if (!(rates.paintUsdPerSquareMeter > 0)) rates.paintUsdPerSquareMeter = defaultSettings.rates.paintUsdPerSquareMeter;
    const height = saved.defaultWallHeight;
    return {
      rates,
      currency: currencies.includes(saved.currency ?? "") ? saved.currency! : defaultSettings.currency,
      exchangeRate: finite(saved.exchangeRate, 1, Number.MIN_VALUE),
      preferredUnit: units.includes(saved.preferredUnit ?? "") ? saved.preferredUnit! : defaultSettings.preferredUnit,
      defaultWallHeight: height && units.includes(height.unit) && finite(height.value, 0, Number.MIN_VALUE) > 0 ? { value: height.value, unit: height.unit } : defaultSettings.defaultWallHeight,
    };
  } catch {
    return defaultSettings;
  }
}

export function saveSettings(settings: Settings) {
  localStorage.setItem(settingsKey, JSON.stringify(settings));
}

// Default wall height expressed in the given unit, rounded to 2 decimals.
export function wallHeightIn(settings: Settings, unit: string) {
  const meters = settings.defaultWallHeight.value * (metersPerUnit[settings.defaultWallHeight.unit] ?? 1);
  return String(Number((meters / (metersPerUnit[unit] ?? 1)).toFixed(2)));
}

export function describeDefaultHeight(settings: Settings) {
  return `${settings.defaultWallHeight.value} ${settings.defaultWallHeight.unit}`;
}
