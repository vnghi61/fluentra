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

/**
 * The learner's own avatar, from the profile read the shell already makes.
 *
 * Same query key as useDisplayName, so this costs no extra request: the account
 * menu was drawing a generic person icon for everybody while the URL sat one
 * field away in a response it had already fetched, and an uploaded avatar
 * appeared nowhere outside the settings screen it was uploaded on.
 */
export function useAvatarUrl(signedIn: boolean): string | undefined {
  const { data } = useQuery({
    queryKey: ["account", "me"],
    queryFn: () => accountApi.getMe(),
    enabled: signedIn,
    staleTime: 5 * 60 * 1000,
  });
  return data?.profile.avatar_url ?? undefined;
}
