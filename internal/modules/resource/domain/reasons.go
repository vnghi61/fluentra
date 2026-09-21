package domain

import "fmt"

// The sentences a learner reads when a resource is rejected or fails.
//
// failure_reason is returned by GET /me/resources/{id}, so it is part of the
// API, and it used to be err.Error(). That handed learners Go error chains - and
// for a refused URL, the private address their hostname resolved to, which is an
// oracle for our internal DNS. The detail still exists; it goes to the log.
const (
	// ReasonIncomplete is a row missing what its kind requires. It should never
	// happen; ck_resources_shape exists so that it cannot.
	ReasonIncomplete = "This resource is incomplete and could not be checked."

	// ReasonEmptyFile is a zero-byte upload.
	ReasonEmptyFile = "The uploaded file is empty."

	// ReasonTypeNotSupported covers both an unsupported type and a file whose
	// contents are not what its declared type says (BR-RESOURCE-04). Telling the
	// two apart helps nobody but someone probing the sniffer.
	ReasonTypeNotSupported = "This file type isn't supported, or the file isn't the type its name says it is."

	// ReasonURLNotAllowed is a URL the SSRF guard refused.
	ReasonURLNotAllowed = "Only public web pages can be imported."

	// ReasonURLUnreachable is a network failure or a 5xx: a 'failed', not a
	// 'rejected', because it says nothing about the link.
	ReasonURLUnreachable = "We couldn't reach this page. Please try again later."

	// ReasonIntentExpired is an upload intent that was never completed.
	ReasonIntentExpired = "The upload was never completed."

	// ReasonValidationStuck is a confirmed upload whose check never finished.
	ReasonValidationStuck = "We couldn't finish checking this file. Please upload it again."
)

// ReasonTooLarge names the limit, which is the one thing a learner needs to fix it.
func ReasonTooLarge() string {
	return fmt.Sprintf("The file is larger than the %d MB limit.", MaxResourceBytes/(1024*1024))
}

// ReasonURLStatus is a 4xx from the remote site. The status code is theirs, not
// ours, so it is safe to show and it tells the learner what happened.
func ReasonURLStatus(code int) string {
	return fmt.Sprintf("The page answered with an error (HTTP %d).", code)
}
