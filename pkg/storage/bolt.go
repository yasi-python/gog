package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketConfigs = []byte("configs")
	bucketStats   = []byte("stats")
	bucketState   = []byte("state")
)

type DB struct {
	db *bolt.DB
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, e := tx.CreateBucketIfNotExists(bucketConfigs); e != nil { return e }
		if _, e := tx.CreateBucketIfNotExists(bucketStats); e != nil { return e }
		if _, e := tx.CreateBucketIfNotExists(bucketState); e != nil { return e }
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

type ConfigRecord struct {
	ID        string `json:"id"`
	Raw       string `json:"raw"`
	Proto     string `json:"proto"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Quarantine bool  `json:"quarantine"`
	Deleted   bool   `json:"deleted"`
}

type StatsRecord struct {
	ID                  string `json:"id"`
	Attempts            int    `json:"attempts"`
	Successes           int    `json:"successes"`
	Failures            int    `json:"failures"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastSuccessUnix     int64  `json:"last_success_unix"`
	LastFailureUnix     int64  `json:"last_failure_unix"`
}

func (d *DB) PutConfig(c ConfigRecord) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfigs)
		j, _ := json.Marshal(c)
		return b.Put([]byte(c.ID), j)
	})
}

func (d *DB) GetConfig(id string) (*ConfigRecord, error) {
	var c ConfigRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketConfigs).Get([]byte(id))
		if v == nil { return errors.New("not_found") }
		return json.Unmarshal(v, &c)
	})
	if err != nil { return nil, err }
	return &c, nil
}

func (d *DB) ListConfigs() ([]ConfigRecord, error) {
	out := []ConfigRecord{}
	err := d.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketConfigs).ForEach(func(k, v []byte) error {
			var c ConfigRecord
			if err := json.Unmarshal(v, &c); err == nil {
				out = append(out, c)
			}
			return nil
		})
	})
	return out, err
}

func (d *DB) PutStats(s StatsRecord) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketStats)
		j, _ := json.Marshal(s)
		return b.Put([]byte(s.ID), j)
	})
}

func (d *DB) GetStats(id string) (*StatsRecord, error) {
	var s StatsRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketStats).Get([]byte(id))
		if v == nil { return errors.New("not_found") }
		return json.Unmarshal(v, &s)
	})
	if err != nil { return nil, err }
	return &s, nil
}

func (d *DB) UpdateStatsForProbe(id string, success bool) (*StatsRecord, error) {
	var s StatsRecord
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketStats)
		v := b.Get([]byte(id))
		if v != nil {
			_ = json.Unmarshal(v, &s)
		} else {
			s = StatsRecord{ID: id}
		}
		s.Attempts++
		now := time.Now().Unix()
		if success {
			s.Successes++
			s.LastSuccessUnix = now
			s.ConsecutiveFailures = 0
		} else {
			s.Failures++
			s.LastFailureUnix = now
			s.ConsecutiveFailures++
		}
		j, _ := json.Marshal(s)
		return b.Put([]byte(id), j)
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) SnapshotConfig(c ConfigRecord, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%d.json", c.ID, time.Now().Unix())
	path := filepath.Join(dir, name)
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", err
	}
	return path, nil
}