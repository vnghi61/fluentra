import { useQuery } from "@tanstack/react-query";

import { accountApi } from "./accountApi";
import { accountKeys } from "./keys";

/**
 * Whether Google is linked, and whether it may be unlinked.
 *
 * A query rather than a component-local read, because the security overview
 * and the social sign-in card both describe it: one cache entry means one
 * request, and an unlink invalidates both at once.
 */
export function useGoogleLinkStatus() {
  return useQuery({
    queryKey: accountKeys.googleLink(),
    queryFn: () => accountApi.getGoogleLinkStatus(),
  });
}
