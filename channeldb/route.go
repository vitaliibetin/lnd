package channeldb

import "github.com/boltdb/bolt"

func (d *DB) PutRoutingTable(routingTable []byte) error {
	return d.store.Update(func (tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(routingManagerBucket)
		if err != nil {
			return err
		}
		return bucket.Put(routingTableKey, routingTable)
	})
}

func (d *DB) FetchRoutingTable() ([]byte, error) {
	var routingTable []byte
	err := d.store.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(routingManagerBucket)
		if bucket == nil {
			return nil
		}
		routingTableBytes := bucket.Get(routingTableKey)
		if routingTableBytes == nil {
			return nil
		}
		routingTable = make([]byte, len(routingTableBytes))
		copy(routingTable, routingTableBytes)
		return nil
	})
	return routingTable, err
}

func (d *DB) DeleteRoutingTable() error {
	return d.store.Update(func (tx *bolt.Tx) error {
		bucket := tx.Bucket(routingManagerBucket)
		if bucket == nil {
			return nil
		}
		return bucket.Delete(routingTableKey)
	})
}