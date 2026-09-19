/** What the play route returns: a short-lived URL and what is left of the quota. */
export interface ListeningPlayResult {
  audio_url: string;
  plays_used: number;
  plays_allowed: number;
  expires_at: string;
}

/** The script, released only once the attempt holding it has been graded. */
export interface ListeningTranscriptResult {
  transcript: string;
}
