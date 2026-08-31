package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/service"
)

// BadgeResponse is one earned badge.
type BadgeResponse struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tier        string    `json:"tier"`
	EarnedAt    time.Time `json:"earned_at"`
}

// StreakResponse is a streak with everything needed to explain it on screen.
type StreakResponse struct {
	Current          int     `json:"current"`
	Longest          int     `json:"longest"`
	LastActiveOn     *string `json:"last_active_on"`
	FreezesAvailable int     `json:"freezes_available"`
	HoursRemaining   int     `json:"hours_remaining"`
}

// QuestResponse is one quest in flight.
type QuestResponse struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Progress    map[string]int `json:"progress"`
	Steps       map[string]int `json:"steps"`
	RewardXP    int            `json:"reward_xp"`
	ExpiresOn   string         `json:"expires_on"`
}

// SummaryResponse is the whole gamification state in one payload.
type SummaryResponse struct {
	TotalXP      int64           `json:"total_xp"`
	Level        int             `json:"level"`
	LevelStartXP int64           `json:"level_start_xp"`
	NextLevelXP  int64           `json:"next_level_xp"`
	XPToday      int64           `json:"xp_today"`
	DailyGoalXP  int             `json:"daily_goal_xp"`
	Streak       StreakResponse  `json:"streak"`
	Badges       []BadgeResponse `json:"badges"`
	Quests       []QuestResponse `json:"quests"`
	League       string          `json:"league"`
}

// LeaderboardEntryResponse is one standing.
//
// Display name only — no email, no avatar, nothing that identifies a learner
// beyond the name they chose to show (BR-GAMIFICATION-07).
type LeaderboardEntryResponse struct {
	Rank        int       `json:"rank"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	XP          int       `json:"xp"`
	IsSelf      bool      `json:"is_self"`
}

// LeaderboardResponse is a league's standings.
type LeaderboardResponse struct {
	Entries []LeaderboardEntryResponse `json:"entries"`
}

// SetDailyGoalRequest changes the XP a day must earn to count.
type SetDailyGoalRequest struct {
	DailyGoalXP int `json:"daily_goal_xp"`
}

// SetLeaderboardOptInRequest opts a learner in or out of being ranked.
type SetLeaderboardOptInRequest struct {
	OptIn bool `json:"opt_in"`
}

// dayString renders a date column, or nil when it is absent.
func dayString(day *time.Time) *string {
	if day == nil || day.IsZero() {
		return nil
	}
	formatted := day.Format(time.DateOnly)
	return &formatted
}

func mapStreak(streak contract.Streak) StreakResponse {
	return StreakResponse{
		Current:          streak.Current,
		Longest:          streak.Longest,
		LastActiveOn:     dayString(streak.LastActiveOn),
		FreezesAvailable: streak.FreezesAvailable,
		HoursRemaining:   streak.HoursRemaining,
	}
}

func mapSummary(summary contract.Summary) SummaryResponse {
	badges := make([]BadgeResponse, 0, len(summary.Badges))
	for _, badge := range summary.Badges {
		badges = append(badges, BadgeResponse{
			Code: badge.Code, Name: badge.Name, Description: badge.Description,
			Tier: badge.Tier, EarnedAt: badge.EarnedAt,
		})
	}

	quests := make([]QuestResponse, 0, len(summary.Quests))
	for _, quest := range summary.Quests {
		quests = append(quests, QuestResponse{
			Code: quest.Code, Name: quest.Name, Description: quest.Description,
			Progress: quest.Progress, Steps: quest.Steps, RewardXP: quest.RewardXP,
			ExpiresOn: quest.ExpiresOn.Format(time.DateOnly),
		})
	}

	return SummaryResponse{
		TotalXP:      summary.TotalXP,
		Level:        summary.Level,
		LevelStartXP: summary.LevelStartXP,
		NextLevelXP:  summary.NextLevelXP,
		XPToday:      summary.XPToday,
		DailyGoalXP:  summary.DailyGoalXP,
		Streak:       mapStreak(summary.Streak),
		Badges:       badges,
		Quests:       quests,
		League:       summary.League,
	}
}

func mapLeaderboard(entries []service.LeaderboardEntry) LeaderboardResponse {
	rows := make([]LeaderboardEntryResponse, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, LeaderboardEntryResponse{
			Rank: entry.Rank, UserID: entry.UserID, DisplayName: entry.DisplayName,
			XP: entry.XP, IsSelf: entry.IsSelf,
		})
	}
	return LeaderboardResponse{Entries: rows}
}
