/** Which greeting the Dashboard opens with, by the learner's local hour. */
export type GreetingKey = "morning" | "afternoon" | "evening";

/**
 * 05:00-11:59 morning, 12:00-17:59 afternoon, otherwise evening (late night
 * included: "good morning" at 2 a.m. reads as a bug).
 */
export function greetingKeyFor(hour: number): GreetingKey {
  if (hour >= 5 && hour < 12) return "morning";
  if (hour >= 12 && hour < 18) return "afternoon";
  return "evening";
}
