package domain

import (
	"math"
)

// The adaptive placement engine — work order 13 §3.4.
//
// Pure: no database, no clock and no randomness, so the same responses in the
// same order always give the same estimate, and the whole engine can be tested
// by simulation.

// PlacementBands are the levels a placement can result in. C2 is not placed.
var PlacementBands = [5]string{LevelA1, LevelA2, LevelB1, LevelB2, LevelC1}

// bandTheta is each band's position on the ability scale.
var bandTheta = [5]float64{-2, -1, 0, 1, 2}

// Placement skills measured by the adaptive part.
const (
	SkillVocabulary = "vocabulary"
	SkillGrammar    = "grammar"
	SkillReading    = "reading"
	SkillListening  = "listening"
	SkillWriting    = "writing"
	SkillSpeaking   = "speaking"
)

// Placement stages, as stored on the session.
const (
	PlacementStageVocabularyGrammar = "vocabulary_grammar"
	PlacementStageReadingListening  = "reading_listening"
	PlacementStageRefine            = "refine"
	PlacementStageDone              = "done"
)

// The stopping rule and the item budget (§2).
const (
	PlacementStopConfidence = 0.80
	PlacementMinResponses   = 8
	PlacementMaxResponses   = 25
	// PlacementStageOneMax is how many vocabulary and grammar responses the first
	// stage takes at most before moving on to reading and listening.
	PlacementStageOneMax = 14
	// PlacementQuestionsPerPassage is how many observations a passage or clip is
	// assumed to add when checking whether the budget allows another one.
	PlacementQuestionsPerPassage = 3
	// maxRefiningItems bounds the passages and clips served after the first two,
	// so an item that yields no observation cannot keep the test running.
	maxRefiningItems = 4
	// skillShrinkage is how many responses a skill needs before its own
	// estimate outweighs the overall one.
	skillShrinkage = 5.0
	// discrimination is the slope of the response model.
	discrimination = 1.7
)

// Observation is one scored response: a single question, or one question of a
// passage or clip.
type Observation struct {
	Skill   string `json:"skill"`
	Band    string `json:"band"`
	Correct bool   `json:"correct"`
	// Options is the number of choices when the question is multiple choice, and
	// zero otherwise. It sets the chance of guessing right.
	Options int `json:"options,omitempty"`
}

// PlacementEstimate is a probability over the five bands and the observations
// that produced it.
type PlacementEstimate struct {
	Posterior    [5]float64    `json:"posterior"`
	Observations []Observation `json:"observations"`
	// Exhausted lists the skills for which no item was left at any band. The
	// planner skips them instead of asking for an item that cannot be served.
	Exhausted []string `json:"exhausted,omitempty"`
}

// NewPlacementEstimate returns the uniform starting estimate.
func NewPlacementEstimate() PlacementEstimate {
	return PlacementEstimate{
		Posterior:    uniformPosterior(),
		Observations: []Observation{},
	}
}

func uniformPosterior() [5]float64 {
	return [5]float64{0.2, 0.2, 0.2, 0.2, 0.2}
}

// BandIndex returns the position of a band in PlacementBands, or -1.
func BandIndex(band string) int {
	for i, b := range PlacementBands {
		if b == band {
			return i
		}
	}
	return -1
}

// ProbabilityCorrect is c + (1 − c) · σ(1.7 · (θ − d)), with c = 1/options for
// multiple choice and 0 otherwise.
func ProbabilityCorrect(theta, difficulty float64, options int) float64 {
	guess := 0.0
	if options > 1 {
		guess = 1.0 / float64(options)
	}
	sigma := 1.0 / (1.0 + math.Exp(-discrimination*(theta-difficulty)))
	return guess + (1.0-guess)*sigma
}

// Observe folds one response into the estimate by Bayes' rule. An observation
// at a band the engine does not place is ignored.
func (e *PlacementEstimate) Observe(obs Observation) {
	index := BandIndex(obs.Band)
	if index < 0 {
		return
	}
	e.Posterior = updated(e.Posterior, index, obs)
	e.Observations = append(e.Observations, obs)
}

func updated(prior [5]float64, band int, obs Observation) [5]float64 {
	var next [5]float64
	var total float64
	for i := range prior {
		p := ProbabilityCorrect(bandTheta[i], bandTheta[band], obs.Options)
		if !obs.Correct {
			p = 1 - p
		}
		next[i] = prior[i] * p
		total += next[i]
	}
	if total <= 0 {
		return prior
	}
	for i := range next {
		next[i] /= total
	}
	return next
}

// Responses is how many scored responses the estimate holds.
func (e PlacementEstimate) Responses() int {
	return len(e.Observations)
}

// MostProbable returns the band with the highest probability and that
// probability. A tie goes to the lower band.
func (e PlacementEstimate) MostProbable() (band int, probability float64) {
	return argmax(e.Posterior)
}

func argmax(p [5]float64) (int, float64) {
	best := 0
	for i := 1; i < len(p); i++ {
		if p[i] > p[best] {
			best = i
		}
	}
	return best, p[best]
}

// Mean is the expected position on the ability scale.
func (e PlacementEstimate) Mean() float64 {
	var mean float64
	for i, p := range e.Posterior {
		mean += p * bandTheta[i]
	}
	return mean
}

// Level is the placed level: the most probable band.
func (e PlacementEstimate) Level() string {
	band, _ := e.MostProbable()
	return PlacementBands[band]
}

// Confident reports whether the stopping rule's confidence half is met: the
// most probable band has reached 0.80 after at least eight responses.
func (e PlacementEstimate) Confident() bool {
	_, probability := e.MostProbable()
	return e.Responses() >= PlacementMinResponses && probability >= PlacementStopConfidence
}

// TargetBand is the band nearest the estimate's mean.
func (e PlacementEstimate) TargetBand() int {
	return clampBand(int(math.Round(e.Mean())) + 2)
}

func clampBand(band int) int {
	return max(0, min(len(PlacementBands)-1, band))
}

func (e PlacementEstimate) exhausted(skill string) bool {
	for _, s := range e.Exhausted {
		if s == skill {
			return true
		}
	}
	return false
}

// MarkExhausted records that a skill has no item left at any band.
func (e *PlacementEstimate) MarkExhausted(skill string) {
	if !e.exhausted(skill) {
		e.Exhausted = append(e.Exhausted, skill)
	}
}

// PlacementProgress counts the items served so far, by skill. A passage or clip
// is one item however many questions it holds.
type PlacementProgress struct {
	Vocabulary int
	Grammar    int
	Reading    int
	Listening  int
}

// PlacementStep is what the engine asks for next.
type PlacementStep struct {
	Done  bool
	Skill string
	Band  string
	Stage string
}

// NextPlacementStep decides the next item, or that the adaptive part is over.
//
// First vocabulary and grammar alternate at the band nearest the estimate until
// the stopping rule would stop or fourteen responses. Then one reading passage
// and one listening clip at the provisional level. Then, while the rule has not
// stopped and the budget allows, another passage or clip one band towards where
// the estimate is least sure.
func NextPlacementStep(e PlacementEstimate, progress PlacementProgress, timeUp bool) PlacementStep {
	if timeUp || e.Responses() >= PlacementMaxResponses {
		return PlacementStep{Done: true, Stage: PlacementStageDone}
	}
	if step, ok := stageOneStep(e, progress); ok {
		return step
	}
	if step, ok := stageTwoStep(e, progress); ok {
		return step
	}
	return refiningStep(e, progress)
}

func stageOneStep(e PlacementEstimate, progress PlacementProgress) (PlacementStep, bool) {
	if progress.Reading+progress.Listening > 0 {
		return PlacementStep{}, false
	}
	if e.Confident() || e.Responses() >= PlacementStageOneMax {
		return PlacementStep{}, false
	}
	vocabulary := !e.exhausted(SkillVocabulary)
	grammar := !e.exhausted(SkillGrammar)
	var skill string
	switch {
	case vocabulary && grammar:
		skill = SkillVocabulary
		if progress.Vocabulary > progress.Grammar {
			skill = SkillGrammar
		}
	case vocabulary:
		skill = SkillVocabulary
	case grammar:
		skill = SkillGrammar
	default:
		return PlacementStep{}, false
	}
	return PlacementStep{
		Skill: skill,
		Band:  PlacementBands[e.TargetBand()],
		Stage: PlacementStageVocabularyGrammar,
	}, true
}

func stageTwoStep(e PlacementEstimate, progress PlacementProgress) (PlacementStep, bool) {
	provisional, _ := e.MostProbable()
	band := PlacementBands[provisional]
	if progress.Reading == 0 && !e.exhausted(SkillReading) {
		return PlacementStep{Skill: SkillReading, Band: band, Stage: PlacementStageReadingListening}, true
	}
	if progress.Listening == 0 && !e.exhausted(SkillListening) {
		return PlacementStep{Skill: SkillListening, Band: band, Stage: PlacementStageReadingListening}, true
	}
	return PlacementStep{}, false
}

func refiningStep(e PlacementEstimate, progress PlacementProgress) PlacementStep {
	done := PlacementStep{Done: true, Stage: PlacementStageDone}
	if e.Confident() || e.Responses()+PlacementQuestionsPerPassage > PlacementMaxResponses {
		return done
	}
	served := progress.Reading + progress.Listening
	if served >= 2+maxRefiningItems {
		return done
	}
	reading := !e.exhausted(SkillReading)
	listening := !e.exhausted(SkillListening)
	var skill string
	switch {
	case reading && listening:
		skill = SkillReading
		if progress.Reading > progress.Listening {
			skill = SkillListening
		}
	case reading:
		skill = SkillReading
	case listening:
		skill = SkillListening
	default:
		return done
	}
	return PlacementStep{
		Skill: skill,
		Band:  PlacementBands[e.towardsUncertainty()],
		Stage: PlacementStageRefine,
	}
}

// towardsUncertainty is one band from the provisional level, on the side holding
// more of the probability that is not on it.
func (e PlacementEstimate) towardsUncertainty() int {
	provisional, _ := e.MostProbable()
	var below, above float64
	for i, p := range e.Posterior {
		switch {
		case i < provisional:
			below += p
		case i > provisional:
			above += p
		}
	}
	if above > below {
		return clampBand(provisional + 1)
	}
	return clampBand(provisional - 1)
}

// BandSearchOrder is the order in which bands are tried for an item: the band
// asked for, then its nearer neighbour — the one on the side of the mean — then
// the other, then further out. A band with no unseen item never stalls a test.
func BandSearchOrder(band string, mean float64) []string {
	start := BandIndex(band)
	if start < 0 {
		start = clampBand(int(math.Round(mean)) + 2)
	}
	first, second := -1, 1
	if mean > bandTheta[start] {
		first, second = 1, -1
	}
	order := []string{PlacementBands[start]}
	for distance := 1; distance < len(PlacementBands); distance++ {
		for _, direction := range []int{first, second} {
			if i := start + direction*distance; i >= 0 && i < len(PlacementBands) {
				order = append(order, PlacementBands[i])
			}
		}
	}
	return order
}

// SkillEstimate is one skill's placed band and how many responses it rests on.
type SkillEstimate struct {
	Band      string `json:"band"`
	Responses int    `json:"responses"`
}

// PerSkill estimates each measured skill from its own responses, drawn towards
// the overall estimate in proportion to how few it had. A skill with no
// responses is absent, never guessed.
func (e PlacementEstimate) PerSkill() map[string]SkillEstimate {
	bySkill := make(map[string][]Observation)
	for _, obs := range e.Observations {
		bySkill[obs.Skill] = append(bySkill[obs.Skill], obs)
	}
	out := make(map[string]SkillEstimate, len(bySkill))
	for skill, observations := range bySkill {
		own := uniformPosterior()
		for _, obs := range observations {
			if index := BandIndex(obs.Band); index >= 0 {
				own = updated(own, index, obs)
			}
		}
		n := float64(len(observations))
		weight := n / (n + skillShrinkage)
		var blended [5]float64
		for i := range blended {
			blended[i] = weight*own[i] + (1-weight)*e.Posterior[i]
		}
		band, _ := argmax(blended)
		out[skill] = SkillEstimate{Band: PlacementBands[band], Responses: len(observations)}
	}
	return out
}

// ProductiveBand maps a graded writing or speaking score to a CEFR band for the
// placement result. The table is recorded in learning/DECISIONS.md.
func ProductiveBand(score, maxScore int) string {
	if maxScore <= 0 {
		return LevelA1
	}
	percent := float64(score) * 100 / float64(maxScore)
	switch {
	case percent < 30:
		return LevelA1
	case percent < 50:
		return LevelA2
	case percent < 70:
		return LevelB1
	case percent < 85:
		return LevelB2
	default:
		return LevelC1
	}
}
