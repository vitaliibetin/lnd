package channeldb

import (
	"bytes"
	"github.com/boltdb/bolt"
	"github.com/BitfuryLightning/tools/rt"
)

func (d *DB) PutRoutingTable(routingTable *rt.RoutingTable) error {
	var buffer bytes.Buffer
	if err := routingTable.Marshall(&buffer); err != nil {
		return err
	}
	return d.store.Update(func (tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(routingManagerBucket)
		if err != nil {
			return err
		}
		return bucket.Put(routingTableKey, buffer.Bytes())
	})
}

func (d *DB) FetchRoutingTable() (*rt.RoutingTable, error) {
	routingTable := rt.NewRoutingTable()
	err := d.store.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(routingManagerBucket)
		if bucket == nil {
			return nil
		}
		routingTableBytes := bucket.Get(routingTableKey)
		if routingTableBytes == nil {
			return nil
		}
		buffer := bytes.NewBuffer(routingTableBytes)
		var err error
		routingTable, err = rt.UnmarshallRoutingTable(buffer)
		return err
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