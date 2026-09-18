import { useEffect, useState } from "react";

/**
 * Counts down from the seconds the server last reported. The server's clock is
 * the one that counts: every new response resets the count to what it says,
 * so a sleeping laptop or a changed system clock cannot buy time.
 */
export function useCountdown(seconds: number, resetKey: string): number {
  const [left, setLeft] = useState(seconds);
  const [key, setKey] = useState(resetKey);

  if (key !== resetKey) {
    setKey(resetKey);
    setLeft(seconds);
  }

  useEffect(() => {
    const timer = setInterval(() => {
      setLeft((current) => Math.max(0, current - 1));
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  return left;
}
