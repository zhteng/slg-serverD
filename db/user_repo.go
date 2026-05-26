package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slg-serverD/data"
)

// user repo
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// CreateUser 创建新用户（事务：插入用户 + 背包 + 建筑 + 部队）
func (r *UserRepo) CreateUser(ctx context.Context, u *data.User, bag *data.Bag, building *data.Building, troops *data.Troops) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			if rErr := tx.Rollback(); rErr != nil {
				log.Printf("CRITICAL: rollback failed after panic: %v", rErr) // 必须记录
			}
			err = fmt.Errorf("panic in transaction: %v", r)
		} else if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				log.Printf("warning: rollback failed on error: %v", rErr)
			}
		}
	}()

	// 插入用户
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (uid, name, password_hash, level, gold, power, alliance_id, city_x, city_y, resources) 
        VALUES (?,?,?,?,?,?,?,?,?,?)`,
		u.Uid, u.Name, u.PasswordHash, u.Level, u.Gold, u.Power, u.AllianceId, u.CityX, u.CityY, "{}")
	if err != nil {
		return err
	}

	// 插入背包
	bagBytes, _ := json.Marshal(bag.Info)
	_, err = tx.ExecContext(ctx, "INSERT INTO bags (uid, info) VALUES (?,?)", u.Uid, string(bagBytes))
	if err != nil {
		return err
	}

	// 插入建筑
	infoBytes, _ := json.Marshal(building.Info)
	queueBytes, _ := json.Marshal(building.Queue)
	_, err = tx.ExecContext(ctx, "INSERT INTO buildings (uid, info, queue, updated_at) VALUES (?,?,?,?)", u.Uid, string(infoBytes), string(queueBytes), building.UpdatedAt)
	if err != nil {
		log.Printf("DB Error - GetUser uid=%d: %v", u.Uid, err)
		return err
	}

	// 插入部队
	tBytes, _ := json.Marshal(troops.Troops)
	dBytes, _ := json.Marshal(troops.Damaged)
	aBytes, _ := json.Marshal(troops.Attack)
	quBytes, _ := json.Marshal(troops.Queue)
	_, err = tx.ExecContext(ctx, `INSERT INTO troops (uid, troops, damaged, attack, queue, update_time, version) 
        VALUES (?,?,?,?,?,?,?)`,
		u.Uid, string(tBytes), string(dBytes), string(aBytes), string(quBytes), troops.UpdateTime, troops.Version)
	if err != nil {
		fmt.Println("Error inserting troops")
		return err
	}

	return tx.Commit()
}

func (r *UserRepo) GetUser(ctx context.Context, uid int64) (*data.User, error) {
	row := r.db.QueryRowContext(ctx, "SELECT uid, name, level, gold, power, alliance_id, city_x, city_y, resources FROM users WHERE uid=?", uid)

	u := &data.User{}
	var resourcesJson string
	err := row.Scan(&u.Uid, &u.Name, &u.Level, &u.Gold, &u.Power, &u.AllianceId, &u.CityX, &u.CityY, &resourcesJson)
	_ = json.Unmarshal([]byte(resourcesJson), &u.Resources)

	if err != nil {
		return nil, err
	}

	return u, nil
}

func (r *UserRepo) SaveUser(ctx context.Context, u *data.User) error {
	infoBytes, _ := json.Marshal(u.Resources)
	_, err := r.db.ExecContext(ctx, `INSERT INTO users (uid, name, level, gold, power, alliance_id, city_x, city_y, resources) , 
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name=?, level=?, gold=?, power=?, alliance_id=?, city_x=?, city_y=?, resources=?`,
		u.Uid, u.Name, u.Level, u.Gold, u.Power, u.AllianceId, u.CityX, u.CityY, infoBytes,
		u.Name, u.Level, u.Gold, u.Power, u.AllianceId, u.CityX, u.CityY, infoBytes)

	return err
}

func (r *UserRepo) GetUserByName(ctx context.Context, name string) (*data.User, error) {
	row := r.db.QueryRowContext(ctx, "SELECT uid, name, password_hash, level, gold, power, alliance_id, city_x, city_y, resources FROM users WHERE name=?", name)
	fmt.Println("SELECT uid, name, password_hash, level, gold, power, alliance_id, city_x, city_y, resources FROM users WHERE name=?", name)
	u := &data.User{}
	var resourcesJson string
	err := row.Scan(&u.Uid, &u.Name, &u.PasswordHash, &u.Level, &u.Gold, &u.Power, &u.AllianceId, &u.CityX, &u.CityY, &resourcesJson)
	if err != nil {
		return nil, err
	}
	u.Resources = make(map[string]int)
	_ = json.Unmarshal([]byte(resourcesJson), &u.Resources)

	return u, err
}

// IsCoordinateOccupied 检查坐标是否已被占用
func (r *UserRepo) IsCoordinateOccupied(ctx context.Context, x, y int) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE city_x=? AND city_y=?", x, y).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
