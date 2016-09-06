package channeldb

import "github.com/boltdb/bolt"

func (d *DB) PutLightningIDToHost(lightningIDToHost map[string]string) error {
	return d.store.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(lightningIDToHostBucket)
		if err != nil {
			return err
		}
		for key, val := range lightningIDToHost {
			err := bucket.Put([]byte(key), []byte(val))
			if err != nil {
				return err
			}
		} 
		return nil
	})
}

func (d *DB) FetchLightningIDToHost() (map[string]string, error) {
	lightningIDToHost := make(map[string]string)
	err := d.store.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lightningIDToHostBucket)
		if bucket == nil {
			return nil
		}
		if err := bucket.ForEach(func(key, val []byte) error {
			lightningIDToHost[string(key)] = string(val)
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
	return lightningIDToHost, err
}

func (d *DB) DeleteLightningIDToHost() error { return nil }