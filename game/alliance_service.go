package game

import (
	"context"
	"errors"
	"slg-serverD/cache"
	"slg-serverD/data"
	"slg-serverD/db"
	"time"
)

type AllianceService struct {
	repo    *db.AllianceRepo
	cache   *cache.AllianceCache
	userSvc *UserService
	lockSvc *LockService // 分布式锁服务
}

func NewAllianceService(repo *db.AllianceRepo, cache *cache.AllianceCache, userSev *UserService, lockSev *LockService) *AllianceService {
	return &AllianceService{repo: repo, cache: cache, userSvc: userSev, lockSvc: lockSev}
}

// Create 创建军团
func (s *AllianceService) Create(ctx context.Context, name string, icon int, leaderUid int64) (*data.Alliance, error) {
	user, err := s.userSvc.Load(ctx, leaderUid)
	if err != nil {
		return nil, err
	}

	if user.AllianceId != 0 {
		return nil, errors.New("already in alliance")
	}

	now := time.Now().Unix()
	a := &data.Alliance{
		Name:        name,
		Icon:        icon,
		Level:       1,
		Notice:      "Welcome!",
		Leader:      leaderUid,
		MemberCount: 1,
		MaxMember:   50,
		JoinType:    data.MemberPosLeader,
		CreateTime:  now,
	}

	m := &data.AllianceMember{
		Uid:        leaderUid,
		Position:   data.MemberPosLeader,
		JoinTime:   now,
		LastOnline: now,
	}

	aid, err := s.repo.CreateAlliance(ctx, a, m)
	if err != nil {
		return nil, err
	}

	a.Id = aid
	user.AllianceId = aid
	if err := s.userSvc.Save(ctx, user); err != nil {
		return nil, err
	}

	_ = s.cache.Set(ctx, a)
	return a, nil
}

// 申请
func (s *AllianceService) Join(ctx context.Context, aid, uid int64) error {
	a, err := s.LoadAlliance(ctx, aid)
	if err != nil {
		return err
	}

	if a.IsDisband == 1 {
		return errors.New("alliance disbanded")
	}

	user, err := s.userSvc.Load(ctx, uid)
	if err != nil {
		return err
	}
	if user.AllianceId != 0 {
		return errors.New("already in alliance")
	}

	if a.MemberCount >= a.MaxMember {
		return errors.New("alliance full")
	}

	// 加入审批
	switch a.JoinType {
	case data.JoinTypeFree:
		return s.directJoin(ctx, a, uid)
	case data.JoinTypeApply:
		return s.sendApply(ctx, aid, uid)
	case data.JoinTypeRefuse:
		return errors.New("alliance refuse new member")
	}

	return nil
}

func (s AllianceService) directJoin(ctx context.Context, a *data.Alliance, uid int64) error {
	now := time.Now().Unix()

	// 成员
	m := &data.AllianceMember{
		Aid:          a.Id,
		Uid:          uid,
		Position:     data.MemberPosNormal,
		Contribution: 0,
		JoinTime:     now,
		LastOnline:   now,
	}

	if err := s.repo.AddMember(ctx, m); err != nil {
		return err
	}

	// 更新成员数量
	a.MemberCount++
	if err := s.repo.UpdateAlliance(ctx, a); err != nil {
		return err
	}
	_ = s.cache.Set(ctx, a)

	// 更新user.aid
	u, _ := s.userSvc.Load(ctx, uid)
	u.AllianceId = a.Id
	_ = s.userSvc.Save(ctx, u)
	return nil
}

// 需要审批
func (s *AllianceService) sendApply(ctx context.Context, aid, uid int64) error {
	apply := &data.AllianceApply{
		Aid:       aid,
		Uid:       uid,
		ApplyTime: time.Now().Unix(),
	}

	return s.repo.AddApply(ctx, apply)
}

// 审批通过 状态state = 1 通过，否则拒绝
func (s *AllianceService) Approved(ctx context.Context, aid, uid, applyId int64, state int) error {
	if state == 1 {

	}

	a, err := s.LoadAlliance(ctx, aid)
	if err != nil {
		return errors.New("alliance not exist")
	}

	_, err = s.userSvc.Load(ctx, uid)
	if err != nil {
		return errors.New("user not exist")
	}

	// 更新
	err = s.directJoin(ctx, a, uid)
	if err != nil {
		return err
	}

	// 删除申请记录
	return s.repo.RemoveApply(ctx, applyId)
}

// 踢出成员，使用分布式锁避免并发冲突，并通过事务保证多表一致性
func (s *AllianceService) KickMember(ctx context.Context, operatorUid, targetUid int64) error {
	// 1. 权限校验：操作者必须是自己军团的 Leader 或 Vic
	opUser, err := s.userSvc.Load(ctx, operatorUid)
	if err != nil {
		return err
	}

	if opUser.AllianceId == 0 {
		return errors.New("not in alliance")
	}

	aid := opUser.AllianceId

	opMember, err := s.repo.GetMember(ctx, aid, operatorUid)
	if err != nil || (opMember.Position != data.MemberPosLeader && opMember.Position != data.MemberPosVice) {
		return errors.New("no permission")
	}

	targetMember, err := s.repo.GetMember(ctx, aid, targetUid)
	if err != nil {
		return errors.New("target not in alliance")
	}

	if targetMember.Position == data.MemberPosLeader {
		return errors.New("cannot kick leader")
	}

	// 2. 加锁：对军团和被踢玩家同时加锁，防止并发踢人或玩家同时操作
	unlock, err := s.lockSvc.LockMulti(ctx, AllianceLockKey(aid), UserLockKey(targetUid))
	if err != nil {
		return errors.New("operation conflict, please retry")
	}
	defer unlock()

	// 3. 执行事务：删除成员、更新成员数、清零玩家军团ID、写日志
	if err := s.repo.KickMemberTx(ctx, aid, targetUid, operatorUid); err != nil {
		return err
	}

	s.cache.Del(ctx, aid)
	s.userSvc.cache.Del(ctx, targetUid)
	s.userSvc.cache.Del(ctx, operatorUid)
	return nil
}

// LoadAlliance 先查缓存，再查 DB
func (s *AllianceService) LoadAlliance(ctx context.Context, aid int64) (*data.Alliance, error) {
	a, err := s.cache.Get(ctx, aid)
	if err != nil {
		a, err = s.repo.GetAlliance(ctx, aid)
		if err != nil {
			return nil, err
		}

		_ = s.cache.Set(ctx, a)
	}

	return a, nil
}
