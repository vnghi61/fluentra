package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	targetTypeBanner = "banner"

	// activeBannerTTL is how long one read of the active banners serves every
	// dashboard on this instance, the same generation the flag cache uses.
	activeBannerTTL = 30 * time.Second

	// activeBannerFetch is how many rows one refresh reads. More than the
	// announcements and slides handed out together, so that a banner ending
	// inside the TTL can be dropped on read without leaving the dashboard one
	// short until the next refresh.
	activeBannerFetch = 30
)

// BannerRepository persists core.banners. It is separate from Repository so
// that the flag and user-management fakes do not have to grow banner methods.
type BannerRepository interface {
	ListBanners(ctx context.Context) ([]admindomain.Banner, error)
	ListActiveBanners(ctx context.Context, at time.Time, limit int32) ([]admindomain.Banner, error)
	GetBanner(ctx context.Context, id uuid.UUID) (admindomain.Banner, error)
	CreateBanner(
		ctx context.Context, in admindomain.BannerInput, image *admindomain.BannerImage, createdBy uuid.UUID,
	) (admindomain.Banner, error)
	UpdateBanner(
		ctx context.Context, id uuid.UUID, in admindomain.BannerInput, image *admindomain.BannerImage,
	) (admindomain.Banner, error)
	DeleteBanner(ctx context.Context, id uuid.UUID) error
}

var errBannersUnavailable = apperr.New(
	apperr.Unavailable, "BANNERS_UNAVAILABLE", "Announcement banners are not available right now.")

// activeBannerCache holds the last read of the active banners for one TTL.
// It is per instance: a write invalidates it here, and other instances catch
// up within one TTL.
type activeBannerCache struct {
	mu        sync.Mutex
	banners   []admindomain.Banner
	expiresAt time.Time
	loaded    bool
}

func (c *activeBannerCache) get(now time.Time) ([]admindomain.Banner, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded || !now.Before(c.expiresAt) {
		return nil, false
	}
	return c.banners, true
}

func (c *activeBannerCache) set(banners []admindomain.Banner, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.banners = banners
	c.expiresAt = now.Add(activeBannerTTL)
	c.loaded = true
}

func (c *activeBannerCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.banners = nil
	c.loaded = false
}

// ListActiveBanners returns the banners a dashboard shows now: at most
// three announcements and six slides, highest priority first, then newest first.
// Served from a 30-second in-process cache; rows that have stopped being live
// since the cache was filled are dropped on the way out. Image URLs are signed
// when the cache fills, so every dashboard in one TTL shares them and the
// browser cache can do its job.
func (s *Service) ListActiveBanners(ctx context.Context) ([]admindomain.Banner, error) {
	if s.banners == nil {
		return []admindomain.Banner{}, nil
	}
	now := s.clock.Now()

	candidates, ok := s.bannerCache.get(now)
	if !ok {
		rows, err := s.banners.ListActiveBanners(ctx, now, activeBannerFetch)
		if err != nil {
			return nil, err
		}
		candidates = s.withImageURLs(ctx, rows)
		s.bannerCache.set(candidates, now)
	}

	// Each kind has its own allowance, so a busy carousel never crowds out an
	// announcement or the other way round. Order is kept within each kind.
	limits := map[admindomain.BannerKind]int{
		admindomain.BannerKindAnnouncement: admindomain.ActiveAnnouncementLimit,
		admindomain.BannerKindSlide:        admindomain.ActiveSlideLimit,
	}
	taken := map[admindomain.BannerKind]int{}
	live := make([]admindomain.Banner, 0, admindomain.ActiveAnnouncementLimit+admindomain.ActiveSlideLimit)
	for _, banner := range candidates {
		kind := banner.Kind
		if kind == "" {
			kind = admindomain.BannerKindAnnouncement
		}
		if taken[kind] >= limits[kind] || !banner.LiveAt(now) {
			continue
		}
		taken[kind]++
		live = append(live, banner)
	}
	return live, nil
}

// ListBanners returns every banner for the admin screen.
func (s *Service) ListBanners(ctx context.Context) ([]admindomain.Banner, error) {
	if s.banners == nil {
		return nil, errBannersUnavailable
	}
	banners, err := s.banners.ListBanners(ctx)
	if err != nil {
		return nil, err
	}
	return s.withImageURLs(ctx, banners), nil
}

// CreateBanner validates and stores a new banner, then audits it (BR-ADMIN-02).
func (s *Service) CreateBanner(
	ctx context.Context, actorID uuid.UUID, in admindomain.BannerInput,
) (admindomain.Banner, error) {
	if s.banners == nil {
		return admindomain.Banner{}, errBannersUnavailable
	}
	clean, err := admindomain.NormalizeBannerInput(in)
	if err != nil {
		return admindomain.Banner{}, err
	}
	image, err := s.resolveBannerImage(ctx, clean, nil)
	if err != nil {
		return admindomain.Banner{}, err
	}
	banner, err := s.banners.CreateBanner(ctx, clean, image, actorID)
	if err != nil {
		return admindomain.Banner{}, err
	}
	s.bannerCache.invalidate()
	s.recordBannerAction(ctx, "admin.banner_created", actorID, banner.ID)
	return s.bannerWithImageURL(ctx, banner), nil
}

// UpdateBanner replaces a banner's editable fields and audits the change.
func (s *Service) UpdateBanner(
	ctx context.Context, actorID, id uuid.UUID, in admindomain.BannerInput,
) (admindomain.Banner, error) {
	if s.banners == nil {
		return admindomain.Banner{}, errBannersUnavailable
	}
	clean, err := admindomain.NormalizeBannerInput(in)
	if err != nil {
		return admindomain.Banner{}, err
	}
	current, err := s.banners.GetBanner(ctx, id)
	if err != nil {
		return admindomain.Banner{}, err
	}
	image, err := s.resolveBannerImage(ctx, clean, current.Image)
	if err != nil {
		return admindomain.Banner{}, err
	}
	banner, err := s.banners.UpdateBanner(ctx, id, clean, image)
	if err != nil {
		return admindomain.Banner{}, err
	}
	s.deleteReplacedBannerImage(ctx, current.Image, banner.Image)
	s.bannerCache.invalidate()
	s.recordBannerAction(ctx, "admin.banner_updated", actorID, banner.ID)
	return s.bannerWithImageURL(ctx, banner), nil
}

// DeleteBanner removes a banner and audits the deletion.
func (s *Service) DeleteBanner(ctx context.Context, actorID, id uuid.UUID) error {
	if s.banners == nil {
		return errBannersUnavailable
	}
	current, err := s.banners.GetBanner(ctx, id)
	if err != nil {
		return err
	}
	if err := s.banners.DeleteBanner(ctx, id); err != nil {
		return err
	}
	s.deleteReplacedBannerImage(ctx, current.Image, nil)
	s.bannerCache.invalidate()
	s.recordBannerAction(ctx, "admin.banner_deleted", actorID, id)
	return nil
}

func (s *Service) recordBannerAction(ctx context.Context, action string, actorID, bannerID uuid.UUID) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, auditcontract.Entry{
		Action:     action,
		TargetType: targetTypeBanner,
		TargetID:   bannerID.String(),
		Meta: map[string]any{
			metaActorID: actorID.String(),
		},
	})
}
