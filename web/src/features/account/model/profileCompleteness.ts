import type {
  LearningProfile,
  UserPreferences,
  UserProfile,
} from "../api/accountApi";

export type CompletenessKey =
  | "basicInfo"
  | "photo"
  | "emailVerified"
  | "learningGoals"
  | "learningPreferences";

export interface CompletenessItem {
  key: CompletenessKey;
  done: boolean;
}

/**
 * The five steps the "personalised experience" card counts, each read from a
 * field the API returns — never a guess. `learning` is null when the learner
 * has no learning profile yet (the API answers 404), undefined while loading.
 */
export function profileCompleteness(
  profile: UserProfile,
  preferences: UserPreferences,
  learning: LearningProfile | null | undefined,
): CompletenessItem[] {
  const p = profile.profile;
  return [
    {
      key: "basicInfo",
      done: Boolean(p.display_name.trim() && p.country && p.date_of_birth),
    },
    { key: "photo", done: Boolean(p.avatar_url) },
    { key: "emailVerified", done: Boolean(profile.email_verified_at) },
    {
      key: "learningGoals",
      done: Boolean(
        learning && (learning.target_level || learning.target_exam !== "none"),
      ),
    },
    {
      key: "learningPreferences",
      done: preferences.practice_level != null,
    },
  ];
}
