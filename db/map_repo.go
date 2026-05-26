package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slg-serverD/data"
	"strings"
	"time"
)

type MapRepo struct {
	db *sql.DB
}

func NewMapRepo(db *sql.DB) *MapRepo {
	return &MapRepo{db: db}
}

// SaveCell 单地块保存
func (r *MapRepo) SaveCell(ctx context.Context, cell *data.MapCell) error {
	dataJSON, err := json.Marshal(cell.Data)
	if err != nil {
		return err
	}

	dataStr := string(dataJSON)
	if dataStr == "null" {
		dataStr = "{}"
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO map_cells (id,x,y,mtype,level,data,oid,name,power,`+"`rank`"+`,alliance,protect,pic) 
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) 
    ON DUPLICATE KEY UPDATE 
        mtype=VALUES(mtype), 
        level=VALUES(level), 
        data=VALUES(data), 
        oid=VALUES(oid), 
        protect=VALUES(protect),
        `+"`rank`"+`=VALUES(`+"`rank`"+`)`,

		cell.ID, cell.X, cell.Y,
		cell.MType, cell.Level, dataStr,
		cell.Owner, cell.Name, cell.Power, cell.Rank,
		cell.Alliance, cell.Protect, cell.Pic,
	)

	return err
}

func (r *MapRepo) LoadCell(ctx context.Context, x, y int) (*data.MapCell, error) {

	return nil, nil
}

const batchSize = 500 // 一次500条

// BatchSave 批量保存
func (r *MapRepo) BatchSave(ctx context.Context, cells []*data.MapCell) error {
	if len(cells) == 0 {
		return errors.New("empty map cells")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := 0; i < len(cells); i += batchSize {
		end := i + batchSize
		if end > len(cells) {
			end = len(cells)
		}
		batch := cells[i:end]

		// 拼接SQL
		var args []any
		var values []string

		for _, cell := range batch {
			dataJSON, _ := json.Marshal(cell.Data)

			values = append(values, "(?,?,?,?,?,?,?,?,?,?,?,?,?)")
			args = append(args, cell.ID, cell.X, cell.Y, cell.MType, cell.Level, string(dataJSON),
				cell.Owner, cell.Name, cell.Power, cell.Rank, cell.Alliance, cell.Protect, cell.Pic)
		}

		sql := `INSERT INTO map_cells (id,x,y,mtype,level,data,oid,name,power,"rank",alliance,protect,pic) VALUES ` + strings.Join(values, ",") + ` ON DUPLICATE KEY UPDATE 
	mtype=VALUES(mtype), level=VALUES(level), data=VALUES(data), oid=VALUES(oid), protect=VALUES(protect)`
		_, err = tx.ExecContext(ctx, sql, args...)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RelocateTx 搬迁事务：更新玩家坐标、清空旧地块、占领新地块
func (r *MapRepo) RelocateTx(ctx context.Context, uid int64, newX, newY int, oldCel, cell *data.MapCell) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 更新玩家坐标
	_, err = tx.ExecContext(ctx, "UPDATE users SET city_x=?, city_y=?, last_relocate_time=? WHERE uid=?", newX, newY, time.Now().Unix(), uid)
	if err != nil {
		return err
	}

	// 保存旧地块（清空归属）
	if err := r.SaveCellTx(ctx, tx, oldCel); err != nil {
		return err
	}

	// 保存新地块（设置归属）
	if err := r.SaveCellTx(ctx, tx, cell); err != nil {
		return err
	}

	return tx.Commit()
}

// saveCellTx 事务内保存地块
func (r *MapRepo) SaveCellTx(ctx context.Context, tx *sql.Tx, cell *data.MapCell) error {
	dataJSON, _ := json.Marshal(cell)
	_, err := tx.ExecContext(ctx, `
        INSERT INTO map_cells (id, x, y, mtype, level, data, oid, name, power, rank, alliance, protect, pic)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE 
            mtype=VALUES(mtype), level=VALUES(level), data=VALUES(data), oid=VALUES(oid), name=VALUES(name), power=VALUES(power), rank=VALUES(rank), alliance=VALUES(alliance), protect=VALUES(protect), pic=VALUES(pic)`,
		cell.ID, cell.X, cell.Y, cell.MType, cell.Level, string(dataJSON), cell.Owner, cell.Name, cell.Power, cell.Rank, cell.Alliance, cell.Protect, cell.Pic)

	if err != nil {
		return err
	}

	return nil
}

// CountCells 返回地图地块总数
func (r *MapRepo) CountCells(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM map_cells").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// LoadAllCells 加载所有地块（用于初始化阶段，数量大时可分页）
func (r *MapRepo) LoadAllCells(ctx context.Context) ([]*data.MapCell, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, x, y, mtype, level, data, oid, name, power, `rank`, alliance, protect, pic FROM map_cells")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []*data.MapCell
	for rows.Next() {
		var cell data.MapCell
		var dataJSON string

		if err := rows.Scan(&cell.ID, &cell.X, &cell.Y, &cell.MType, &cell.Level,
			&dataJSON, &cell.Owner, &cell.Name, &cell.Power, &cell.Rank, &cell.Alliance, &cell.Protect, &cell.Pic); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(dataJSON), &cell.Data); err != nil {
			return nil, err
		}
		cells = append(cells, &cell)
	}

	return cells, nil
}
