package engine_util

import (
	"bytes"

	"github.com/coocood/badger"
)

func DeleteRange(db *badger.DB, startKey, endKey []byte) error {
	batch := new(WriteBatch)
	txn := db.NewTransaction(false)
	defer txn.Discard()
	for _, cf := range CFs {
		deleteRangeCF(txn, batch, cf, startKey, endKey)
	}

	return batch.WriteToKV(db)
}

func deleteRangeCF(txn *badger.Txn, batch *WriteBatch, cf string, startKey, endKey []byte) {
	if len(endKey) == 0 {
		panic("invalid end key")
	}

	it := NewCFIterator(cf, txn)
	for it.Seek(startKey); it.Valid(); it.Next() {
		item := it.Item()
		key := item.KeyCopy(nil)
		if exceedEndKey(key, endKey) {
			break
		}
		batch.DeleteCF(cf, key)
	}
	defer it.Close()
}

func exceedEndKey(current, endKey []byte) bool {
	return bytes.Compare(current, endKey) >= 0
}
