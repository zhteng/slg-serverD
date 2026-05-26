package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"slg-serverD/data"
)

// bag_repo
type BagRepo struct {
	db *sql.DB
}

func NewBagRepo(db *sql.DB) *BagRepo {
	return &BagRepo{db: db}
}

func (r *BagRepo) GetBag(ctx context.Context, uid int64) (*data.Bag, error) {
	row := r.db.QueryRowContext(ctx, "SELECT uid, info FROM bags WHERE uid=?", uid)

	b := &data.Bag{}
	var infoJson string
	err := row.Scan(&b.Uid, &infoJson)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(infoJson), &b.Info)
	return b, nil
}

func (r BagRepo) SaveBag(ctx context.Context, bag *data.Bag) error {
	infoBytes, _ := json.Marshal(bag.Info)
	_, err := r.db.ExecContext(ctx, "INSERT INTO bags (uid, info) VALUES (?,?) ON DUPLICATE KEY UPDATE info=?",
		bag.Uid, string(infoBytes), string(infoBytes))
	
	return err
}
