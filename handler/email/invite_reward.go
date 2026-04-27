package emailhandler

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

func (h *EmailHandler) handleInviteReward(ctx context.Context, invitee *model.User, requestIP, userAgent string) {
	if !h.shouldGrantInviteReward(invitee) {
		return
	}
	if h.sameInviteIP(invitee, requestIP) {
		h.writeInviteRewardSkippedAudit(ctx, invitee, requestIP, userAgent)
		return
	}

	reward := resolveInviteReward(h.settings)
	if err := h.points.AddPoints(ctx, invitee.InvitedByUserID, model.PointTypeFree, reward, model.PointKindInviteReward, "invite reward", ""); err != nil {
		logger.L.Warn().
			Err(err).
			Str("inviter_id", invitee.InvitedByUserID).
			Str("invitee_id", invitee.ID).
			Msg("failed to grant invite reward")
		return
	}

	_ = handlercommon.WriteAudit(ctx, h.audit, &model.AuditLog{
		UserID:     invitee.InvitedByUserID,
		Action:     model.AuditInviteRewardGranted,
		Resource:   "user",
		ResourceID: invitee.ID,
		IP:         requestIP,
		UserAgent:  userAgent,
		Status:     "success",
		Detail:     fmt.Sprintf("granted %d points", reward),
	})
}

func (h *EmailHandler) shouldGrantInviteReward(invitee *model.User) bool {
	return invitee.InvitedByUserID != "" && h.settings.GetBool(store.SettingInviteRewardEnabled, true)
}

func (h *EmailHandler) sameInviteIP(invitee *model.User, requestIP string) bool {
	return invitee.InviteIP != "" && requestIP != "" && invitee.InviteIP == requestIP
}

func (h *EmailHandler) writeInviteRewardSkippedAudit(ctx context.Context, invitee *model.User, requestIP, userAgent string) {
	_ = handlercommon.WriteAudit(ctx, h.audit, &model.AuditLog{
		UserID:     invitee.InvitedByUserID,
		Action:     model.AuditInviteRewardSkipped,
		Resource:   "user",
		ResourceID: invitee.ID,
		IP:         requestIP,
		UserAgent:  userAgent,
		Status:     "success",
		Detail:     "invite reward skipped due to same ip",
	})
}

func resolveInviteReward(settings *store.SettingStore) int64 {
	minReward := parseInviteRewardBound(settings.Get(store.SettingInviteRewardMin, "1"), 1)
	maxReward := parseInviteRewardBound(settings.Get(store.SettingInviteRewardMax, "5"), 5)
	if minReward > maxReward {
		minReward, maxReward = maxReward, minReward
	}
	if maxReward == minReward {
		return minReward
	}
	return minReward + rand.Int64N(maxReward-minReward+1)
}

func parseInviteRewardBound(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	if v < 1 {
		return 1
	}
	if v > 5 {
		return 5
	}
	return v
}
