package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slg-serverD/data"
)

type TroopsRepo struct {
	db *sql.DB
}

func NewTroopsRepo(db *sql.DB) *TroopsRepo {
	return &TroopsRepo{db: db}
}

// 获取部队
func (r *TroopsRepo) GetTroops(ctx context.Context, uid int64) (*data.Troops, error) {
	row := r.db.QueryRowContext(ctx, `SELECT uid, troops, damaged, attack, queue, update_time, version FROM troops WHERE uid = ?`, uid)

	t := &data.Troops{}
	var troopsJson, damagedJson, attackJson, queueJson string
	err := row.Scan(&t.Uid, &troopsJson, &damagedJson, &attackJson, &queueJson, &t.UpdateTime, &t.Version)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(troopsJson), &t.Troops)
	_ = json.Unmarshal([]byte(damagedJson), &t.Damaged)
	_ = json.Unmarshal([]byte(attackJson), &t.Attack)
	_ = json.Unmarshal([]byte(queueJson), &t.Queue)

	return t, nil
}

// 插入或者更新部队
func (r TroopsRepo) SaveTroops(ctx context.Context, t *data.Troops) error {
	if t.Queue == nil {
		t.Queue = []*data.SoldierQueue{}
	}

	troopsBytes, _ := json.Marshal(t.Troops)
	damagedBytes, _ := json.Marshal(t.Damaged)
	attackBytes, _ := json.Marshal(t.Attack)
	queueBytes, err := json.Marshal(t.Queue)

	if err != nil {
		return fmt.Errorf("marshal queue failed: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `INSERT INTO troops (uid, troops, damaged, attack, queue, update_time, version) 
        VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE troops=?, damaged=?, attack=?, queue=?, update_time=?, version=?`,
		t.Uid, troopsBytes, damagedBytes, attackBytes, queueBytes, t.UpdateTime, t.Version,
		troopsBytes, damagedBytes, attackBytes, queueBytes, t.UpdateTime, t.Version)

	return err
}

func (r *TroopsRepo) GetTroopsWithActiveQueue(ctx context.Context) ([]*data.Troops, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT uid, queue FROM troops WHERE queue != '[]' AND queue != '' AND queue IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var troops []*data.Troops
	for rows.Next() {
		t := &data.Troops{}
		var queueJson string
		if err := rows.Scan(&t.Uid, &queueJson); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(queueJson), &t.Queue); err != nil {
			continue
		}
		troops = append(troops, t)
	}

	return troops, nil
}

// GetTroopsWithActiveMarch 返回所有有活跃行军（attack 不为空）的玩家数据
func (r *TroopsRepo) GetTroopsWithActiveMarch(ctx context.Context) (map[int64]map[string]*data.MarchAttack, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT uid, attack FROM troops WHERE attack != '{}' AND attack IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]map[string]*data.MarchAttack)
	for rows.Next() {
		var uid int64
		var attackJson string
		if err := rows.Scan(&uid, &attackJson); err != nil {
			continue
		}

		var attacks map[string]*data.MarchAttack
		if err := json.Unmarshal([]byte(attackJson), &attacks); err != nil {
			continue
		}
		
		if len(attacks) > 0 {
			result[uid] = attacks
		}
	}

	return result, nil
}
