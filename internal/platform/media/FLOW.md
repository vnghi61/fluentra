---
module: media
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@platform-team"
schema: content
tables: [media_derivatives, transcripts, tts_cache]
depends_on: [storage, job, ai, telemetry]
depended_on_by: [speaking, listening, content, vocabulary, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# media — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Speaking attempt pipeline



```mermaid
sequenceDiagram
    autonumber
    participant J as job queue
    participant W as worker (media)
    participant S as storage
    participant F as ffmpeg
    participant A as ASR provider
    participant P as pronunciation provider

    J->>W: speaking.transcribe_and_score { asset_id }
    W->>S: GET raw upload
    W->>W: sniff magic bytes, probe duration
    alt invalid or too long
        W->>W: fail with a user-safe reason
    else valid
        W->>F: transcode → 16 kHz mono WAV
        W->>F: transcode → 64 kbps Opus (playback)
        W->>F: extract waveform peaks
        W->>S: PUT derivatives
        W->>A: transcribe (16 kHz WAV)
        A-->>W: transcript + word timings + confidence
        alt confidence below threshold
            W->>W: TRANSCRIPTION_LOW_CONFIDENCE — ask the learner to re-record
        else
            W->>P: assess against the reference text
            P-->>W: phoneme accuracy, fluency, completeness
            W->>W: persist transcript + scores; publish media.transcribed
            W->>S: DELETE raw upload
        end
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
