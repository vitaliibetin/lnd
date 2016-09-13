package channeldb

import (
	"github.com/boltdb/bolt"
	"github.com/BitfuryLightning/tools/network/idh"
)

func (d *DB) PutNetworkInfo(info idh.LightningIDToHost) error {
	return d.store.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(networkManagerBucket)
		if err != nil {
			return err
		}
		for key, val := range info {
			err := bucket.Put([]byte(key), []byte(val))
			if err != nil {
				return err
			}
		} 
		return nil
	})
}

func (d *DB) FetchNetworkInfo() (idh.LightningIDToHost, error) {
	info := make(idh.LightningIDToHost)
	err := d.store.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(networkManagerBucket)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(key, val []byte) error {
			info[string(key)] = string(val)
			return nil
		})
	})
	return info, err
}

func (d *DB) DeleteNetworkInfo() error { 
	return d.store.Update(func (tx *bolt.Tx) error {
		bucket := tx.Bucket(networkManagerBucket)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(key, val []byte) error {
			if err := bucket.Delete(key); err != nil {
				return err
			}
			return nil
		})
	})
}