package storage

import (
	"fmt"
	"sort"
	"strings"
)

// r2EndpointSuffix identifies a Cloudflare R2 S3 endpoint.
const r2EndpointSuffix = "r2.cloudflarestorage.com"

// r2Regions are the only values R2 accepts in a credential scope. Anything else
// is refused — with an InvalidRegionName, at 400, on the upload itself.
var r2Regions = map[string]struct{}{
	"auto": {}, "wnam": {}, "enam": {}, "weur": {}, "eeur": {}, "apac": {}, "oc": {},
}

// ValidateRegion rejects a region the configured endpoint will not accept.
//
// It exists because the failure it prevents is invisible until the worst
// possible moment. A deployment with `S3_REGION=ap-southeast-1` against R2
// starts cleanly, serves every request, and issues presigned URLs that look
// perfectly well-formed — the region only appears in the credential scope, and
// nothing reads it until the browser tries to spend the URL. R2 then answers
// 400 InvalidRegionName in a cross-origin response the page is not allowed to
// read, so the learner gets an upload that fails for no stated reason.
//
// Worse, it fails *last*: R2 checks the signature and the CORS policy before it
// looks at the region, so this is the error that surfaces only after every
// other thing has been fixed. That is a long way to walk for a typo in an
// environment variable, and one boot-time check ends it.
//
// Only R2 endpoints are constrained. AWS S3 takes real region names and MinIO
// takes anything, so there is nothing to validate for either.
func ValidateRegion(endpoint, region string) error {
	if !strings.Contains(strings.ToLower(endpoint), r2EndpointSuffix) {
		return nil
	}
	if _, ok := r2Regions[strings.ToLower(strings.TrimSpace(region))]; ok {
		return nil
	}

	valid := make([]string, 0, len(r2Regions))
	for name := range r2Regions {
		valid = append(valid, name)
	}
	sort.Strings(valid)

	return fmt.Errorf(
		"s3.region %q is not valid for Cloudflare R2 (endpoint %q): use one of %s — "+
			"`auto` unless you have a reason to pin a location",
		region, endpoint, strings.Join(valid, ", "))
}
