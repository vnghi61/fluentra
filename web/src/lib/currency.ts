/**
 * VND as Vietnamese readers write it: the symbol after the amount
 * ("1.250.000 ₫"), joined by a no-break space so it never wraps alone.
 */
export const formatVnd = (value: number): string =>
  `${value.toLocaleString("vi-VN")}\u00a0₫`;
