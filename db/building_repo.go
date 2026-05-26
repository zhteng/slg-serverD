package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"slg-serverD/data"
	"time"
)

// buildingColumns 所有建筑字段名（按表字段顺序）
var buildingColumns = []string{"b1", "b2", "b3", "b4", "b5", "b6"}

type BuildingRepo struct {
	db *sql.DB
}

func NewBuildingRepo(db *sql.DB) *BuildingRepo {
	return &BuildingRepo{db: db}
}

// 根据 uid 加载建筑数据
func (r *BuildingRepo) GetBuilding(ctx context.Context, uid int64) (*data.Building, error) {
	row := r.db.QueryRowContext(ctx, "SELECT uid, info, queue, updated_at FROM buildings WHERE uid=?", uid)

	b := &data.Building{}
	var infoJSON, queueJSON string

	err := row.Scan(&b.Uid, &infoJSON, &queueJSON, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}

	b.Info = make(map[string]int)
	b.Queue = make(map[string]int64)
	if infoJSON != "" {
		_ = json.Unmarshal([]byte(infoJSON), &b.Info)
	}

	if queueJSON != "" {
		_ = json.Unmarshal([]byte(queueJSON), &b.Queue)
	}

	return b, nil
}

// SaveBuilding 插入或更新建筑数据
func (r *BuildingRepo) SaveBuilding(ctx context.Context, b *data.Building) error {
	b.UpdatedAt = time.Now().Unix()
	infoBytes, _ := json.Marshal(b.Info)
	queueBytes, _ := json.Marshal(b.Queue)

	_, err := r.db.ExecContext(ctx, "INSERT INTO buildings (uid, info, queue, updated_at) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE info=?, queue=?, updated_at=?",
		b.Uid, string(infoBytes), string(queueBytes), b.UpdatedAt,
		string(infoBytes), string(queueBytes), b.UpdatedAt,
	)
	return err
}

func (r *BuildingRepo) GetBuildingsWithActiveQueue(ctx context.Context) ([]*data.Building, error) {
	// queue 为空字符串或 "{}" 视为无队列
	rows, err := r.db.QueryContext(ctx, "SELECT uid, info, queue, updated_at FROM buildings WHERE queue != '' AND queue != '{}'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	building := make([]*data.Building, 0)
	for rows.Next() {
		b := &data.Building{}
		var infoJson, qJson string
		if err := rows.Scan(&b.Uid, &infoJson, &qJson, &b.UpdatedAt); err != nil {
			continue
		}

		b.Info = make(map[string]int)
		b.Queue = make(map[string]int64)

		_ = json.Unmarshal([]byte(infoJson), &b.Info)
		_ = json.Unmarshal([]byte(qJson), &b.Queue)

		building = append(building, b)
	}

	return building, nil
}
