package httpx

import (
	"net/http"
	"strings"
)

// CORS returns middleware that answers cross-origin requests only for origins
// on an explicit allowlist.
//
// The allowlist is the whole of the policy: an origin that is not on it gets no
// CORS headers at all, which leaves the browser's same-origin policy in force.
// There is no wildcard — `Access-Control-Allow-Origin: *` is incompatible with
// credentialed requests, and echoing the caller's origin is what a wildcard
// effectively does, so both are refused by construction here.
//
// `Access-Control-Allow-Credentials` is set for allowed origins because the
// refresh operation reads a cookie. The browser only honours credentials for an
// exact, allowlisted origin, which is exactly the set this middleware admits.
//
// In the same-origin deployment (the SPA served beside the API behind nginx)
// the browser sends no Origin header and this middleware is a no-op. It exists
// for the local dev split (Vite on 5173, API on 8080) and for any future
// cross-origin client, and it is the only code that reads CORS_ALLOWED_ORIGINS.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(writer, request)
				return
			}
			if _, ok := allowed[origin]; !ok {
				// Not allowlisted: no headers, and the browser blocks the read.
				next.ServeHTTP(writer, request)
				return
			}

			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Add("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Credentials", "true")

			// A preflight asks whether the actual request is allowed. It carries
			// no credentials and no body, and answering it directly is cheaper
			// and clearer than letting it fall through to a handler that was not
			// written for it.
			if request.Method == http.MethodOptions && request.Header.Get("Access-Control-Request-Method") != "" {
				writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				if requested := request.Header.Get("Access-Control-Request-Headers"); requested != "" {
					writer.Header().Set("Access-Control-Allow-Headers", requested)
				}
				writer.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}
