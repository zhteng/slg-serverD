package db

import (
	"context"
	"database/sql"
	"fmt"
	"slg-serverD/data"
	"strings"
	"time"
)

// alliance_repo
type AllianceRepo struct {
	db *sql.DB
}

func NewAllianceRepo(db *sql.DB) *AllianceRepo {
	return &AllianceRepo{db: db}
}

// CreateAlliance 创建军团，同时插入军团记录和盟主成员记录（事务）
func (r *AllianceRepo) CreateAlliance(ctx context.Context, a *data.Alliance, member *data.AllianceMember) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `INSERT INTO alliances (name, icon, level, notice, leader, member_count, max_member, join_type, create_time, is_disband) 
        VALUES (?,?,?,?,?,?,?,?,?,?)`,
		a.Name, a.Icon, a.Level, a.Notice, a.Leader, a.MemberCount, a.MaxMember, a.JoinType, a.CreateTime, a.IsDisband)

	if err != nil {
		return 0, fmt.Errorf("插入联盟失败: %w", err)
	}

	aid, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取联盟ID失败: %w", err)
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO alliance_members (aid, uid, position, contribution, join_time, last_online, quit_cd_time) VALUES (?,?,?,?,?,?,?)",
		aid, member.Uid, member.Position, member.Contribution, member.JoinTime, member.LastOnline, member.QuitCdTime)
	if err != nil {
		return 0, fmt.Errorf("插入成员失败: %w", err)
	}

	// 更新玩家军团ID
	_, err = tx.ExecContext(ctx, `UPDATE users SET alliance_id=? WHERE uid=?`, aid, a.Leader)
	if err != nil {
		return 0, fmt.Errorf("更新成员user.aid失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %w", err)
	}

	return aid, nil
}

// GetAlliance 查询军团详情
func (r AllianceRepo) GetAlliance(ctx context.Context, aid int64) (*data.Alliance, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, icon, level, notice, leader, member_count, max_member, join_type, create_time, is_disband FROM alliances WHERE id=?", aid)

	a := &data.Alliance{}
	err := row.Scan(&a.Id, &a.Name, &a.Icon, &a.Level, &a.Notice, &a.Leader, &a.MemberCount, &a.MaxMember, &a.JoinType, &a.CreateTime, &a.IsDisband)
	return a, err
}

// UpdateAlliance 更新军团信息
func (r *AllianceRepo) UpdateAlliance(ctx context.Context, a *data.Alliance) error {
	// 1. 动态构建 SET 子句，只更新非零值
	var setClauses []string
	var args []interface{}

	if a.Name != "" {
		setClauses = append(setClauses, "name = ?")
		args = append(args, a.Name)
	}

	if a.Icon > 0 {
		setClauses = append(setClauses, "icon = ?")
		args = append(args, a.Icon)
	}

	if a.Level > 0 {
		setClauses = append(setClauses, "level = ?")
		args = append(args, a.Level)
	}

	setClauses = append(setClauses, "notice = ?", "leader = ?", "member_count = ?", "max_member = ?", "join_type = ?", "is_disband = ?")
	args = append(args, a.Notice, a.Leader, a.MemberCount, a.MaxMember, a.JoinType, a.IsDisband)

	if len(setClauses) == 0 {
		return nil
	}

	// 3. 拼接 SQL
	// 注意：WHERE id=? 的参数要追加到 args 后面
	args = append(args, a.Id)

	query := fmt.Sprintf("UPDATE alliances SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("更新联盟失败: %w", err)
	}

	row, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if row == 0 {
		return fmt.Errorf("联盟不存在 (ID: %d): %w", a.Id, sql.ErrNoRows)
	}

	return nil
}

// AddMember 添加成员
func (r AllianceRepo) AddMember(ctx context.Context, m *data.AllianceMember) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	go func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 添加到member 表
	_, err = tx.ExecContext(ctx, "INSERT INTO alliance_members (aid, uid, position, contribution, join_time, last_online, quit_cd_time) VALUES (?,?,?,?,?,?,?)",
		m.Aid, m.Uid, m.Position, m.Contribution, m.JoinTime, m.LastOnline, m.QuitCdTime)

	if err != nil {
		return fmt.Errorf("插入联盟失败: %w", err)
	}

	// 更新 user aid
	var result sql.Result
	result, err = tx.ExecContext(ctx, "UPDATE users SET alliance_id = ? WHERE uid = ?", m.Aid, m.Uid)
	if err != nil {
		return fmt.Errorf("更新成员user.aid失败: %w", err)
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// RemoveMember 删除成员
func (r AllianceRepo) RemoveMember(ctx context.Context, aid int64, uid int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM alliance_members WHERE aid=? AND uid=?", aid, uid)

	// 更新user.aid

	return err
}

// GetMember 查询成员
func (r AllianceRepo) GetMember(ctx context.Context, aid, uid int64) (*data.AllianceMember, error) {
	row := r.db.QueryRowContext(ctx, "SELECT aid, uid, position, contribution, join_time, last_online, quit_cd_time FROM alliance_members WHERE aid=? AND uid=?", aid, uid)
	m := &data.AllianceMember{}
	err := row.Scan(&m.Aid, &m.Uid, &m.Position, &m.Contribution, &m.JoinTime, &m.LastOnline, &m.QuitCdTime)
	return m, err
}

// UpdateMember 更新成员信息
func (r *AllianceRepo) UpdateMember(ctx context.Context, m *data.AllianceMember) error {
	_, err := r.db.ExecContext(ctx, "UPDATE alliance_members SET position=?, contribution=?, last_online=?, quit_cd_time=? WHERE aid=? AND uid=?",
		m.Position, m.Contribution, m.LastOnline, m.QuitCdTime, m.Aid, m.Uid)

	return err
}

// KickMemberTx 踢出成员事务：删除成员、更新成员数、清除玩家军团ID、写日志
// 所有写操作在一个事务中完成，保证原子性
func (r *AllianceRepo) KickMemberTx(ctx context.Context, aid, uid, operatorUid int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	go func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. 删除成员
	if _, err := tx.ExecContext(ctx, "DELETE FROM alliance_members WHERE aid=? AND uid=?", aid, uid); err != nil {
		return err
	}
	// 2. 军团成员数 -1
	if _, err := tx.ExecContext(ctx, "UPDATE alliances SET member_count = member_count - 1 WHERE id=?", aid); err != nil {
		return err
	}
	// 3. 将玩家的 alliance_id 置为 0
	if _, err := tx.ExecContext(ctx, "UPDATE users SET alliance_id = 0 WHERE uid=?", uid); err != nil {
		return err
	}
	// 4. 插入踢人日志
	_, err = tx.ExecContext(ctx, "INSERT INTO alliance_logs (aid, uid, target_uid, type, content, log_time) VALUES (?,?,?,?,?,?)",
		aid, operatorUid, uid, data.LogKick, "kicked out", time.Now().Unix())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// 申请相关
func (r *AllianceRepo) AddApply(ctx context.Context, apply *data.AllianceApply) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO alliance_applies (aid, uid, apply_time) VALUES (?,?,?)", apply.Aid, apply.Uid, apply.ApplyTime)
	return err
}

func (r *AllianceRepo) RemoveApply(ctx context.Context, applyId int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM alliance_applies WHERE apply_id=?", applyId)
	return err
}

func (r AllianceRepo) GetApplyById(ctx context.Context, applyId int64) (*data.AllianceApply, error) {
	row := r.db.QueryRowContext(ctx, "SELECT apply_id, aid, uid, apply_time FROM alliance_applies WHERE id=?", applyId)

	a := &data.AllianceApply{}
	err := row.Scan(&a.Id, &a.Aid, &a.Uid, &a.ApplyTime)
	return a, err
}
