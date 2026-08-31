import { useQuery } from "@tanstack/react-query";

import { accountApi } from "../api/accountApi";

/**
 * The learner's own name, for the places that address them directly.
 *
 * A separate small query rather than a field on the auth store: the store holds
 * what the access token carries — an id and a role — and a display name is
 * profile data that changes without the token changing. Reading it here means
 * renaming yourself in settings updates the greeting, because both go through
 * the same cache key.
 *
 * Returns undefined while loading, for a signed-out visitor, and when the read
 * fails. Every caller has to have something to show without it anyway.
 */
export function useDisplayName(signedIn: boolean): string | undefined {
  const { data } = useQuery({
    queryKey: ["account", "me"],
    queryFn: () => accountApi.getMe(),
    enabled: signedIn,
    staleTime: 5 * 60 * 1000,
  });
  return data?.profile.display_name;
}
