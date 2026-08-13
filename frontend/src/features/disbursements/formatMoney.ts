import type { components } from "@/api/generated/schema";

export type Currency = components["schemas"]["Currency"];
export type Worker = components["schemas"]["Worker"];

const decimalAmountPattern = /^(\d+)\.(\d{2})$/;

export function formatMoney(amount: string, currency: Currency): string {
  return formatMinorUnits(parseMinorUnits(amount), currency);
}

export function totalsByCurrency(workers: readonly Worker[]): Map<Currency, bigint> {
  const totals = new Map<Currency, bigint>();
  for (const worker of workers) {
    totals.set(worker.currency, (totals.get(worker.currency) ?? 0n) + parseMinorUnits(worker.amount));
  }
  return totals;
}

export function formatMinorUnits(minorUnits: bigint, currency: Currency): string {
  const majorUnits = minorUnits / 100n;
  const fractionalUnits = (minorUnits % 100n).toString().padStart(2, "0");
  const majorAmount = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(majorUnits);

  return `${majorAmount}.${fractionalUnits}`;
}

function parseMinorUnits(amount: string): bigint {
  const match = decimalAmountPattern.exec(amount);
  if (!match) {
    throw new Error(`Invalid decimal money value: ${amount}`);
  }

  const majorUnits = match[1];
  const fractionalUnits = match[2];
  if (majorUnits === undefined || fractionalUnits === undefined) {
    throw new Error(`Invalid decimal money value: ${amount}`);
  }
  return BigInt(majorUnits) * 100n + BigInt(fractionalUnits);
}
