package engine_util

import (
	"github.com/coocood/badger"
	"github.com/golang/protobuf/proto"
	"github.com/pingcap/errors"
)

type WriteBatch struct {
	entries       []*badger.Entry
	size          int
	safePoint     int
	safePointSize int
	safePointUndo int
}

const (
	CF_DEFAULT string = "default"
	CF_WRITE   string = "write"
	CF_LOCK    string = "lock"
)

var CFs [3]string = [3]string{CF_DEFAULT, CF_WRITE, CF_LOCK}

func (wb *WriteBatch) Len() int {
	return len(wb.entries)
}

// TODO: make it `SetMeta`
func (wb *WriteBatch) Set(key, val []byte) {
	wb.entries = append(wb.entries, &badger.Entry{
		Key:   key,
		Value: val,
	})
	wb.size += len(key) + len(val)
}

func (wb *WriteBatch) SetCF(cf string, key, val []byte) {
	wb.entries = append(wb.entries, &badger.Entry{
		Key:   append([]byte(cf+"_"), key...),
		Value: val,
	})
	wb.size += len(key) + len(val)
}

// TODO: make it `DeleteMeta`
func (wb *WriteBatch) Delete(key []byte) {
	wb.entries = append(wb.entries, &badger.Entry{
		Key: key,
	})
	wb.size += len(key)
}

func (wb *WriteBatch) DeleteCF(cf string, key []byte) {
	wb.entries = append(wb.entries, &badger.Entry{
		Key: append([]byte(cf+"_"), key...),
	})
	wb.size += len(key)
}

// TODO: remove it
func (wb *WriteBatch) SetMsg(key []byte, msg proto.Message) error {
	val, err := proto.Marshal(msg)
	if err != nil {
		return errors.WithStack(err)
	}
	wb.Set(key, val)
	return nil
}

func (wb *WriteBatch) SetSafePoint() {
	wb.safePoint = len(wb.entries)
	wb.safePointSize = wb.size
}

func (wb *WriteBatch) RollbackToSafePoint() {
	wb.entries = wb.entries[:wb.safePoint]
	wb.size = wb.safePointSize
}

// WriteToKV flush WriteBatch to DB by two steps:
// 	1. Write entries to badger. After save ApplyState to badger, subsequent regionSnapshot will start at new raft index.
//	2. Update lockStore, the date in lockStore may be older than the DB, so we need to restore then entries from raft log.
func (wb *WriteBatch) WriteToKV(db *badger.DB) error {
	if len(wb.entries) > 0 {
		err := db.Update(func(txn *badger.Txn) error {
			for _, entry := range wb.entries {
				var err1 error
				if len(entry.UserMeta) == 0 && len(entry.Value) == 0 {
					err1 = txn.Delete(entry.Key)
				} else {
					err1 = txn.SetEntry(entry)
				}
				if err1 != nil {
					return err1
				}
			}
			return nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (wb *WriteBatch) WriteToRaft(db *badger.DB) error {
	if len(wb.entries) > 0 {
		err := db.Update(func(txn *badger.Txn) error {
			var err1 error
			for _, entry := range wb.entries {
				if len(entry.Value) == 0 {
					err1 = txn.Delete(entry.Key)
				} else {
					err1 = txn.SetEntry(entry)
				}
				if err1 != nil {
					return err1
				}
			}
			return nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (wb *WriteBatch) MustWriteToKV(db *badger.DB) {
	err := wb.WriteToKV(db)
	if err != nil {
		panic(err)
	}
}

func (wb *WriteBatch) MustWriteToRaft(db *badger.DB) {
	err := wb.WriteToRaft(db)
	if err != nil {
		panic(err)
	}
}

func (wb *WriteBatch) Reset() {
	wb.entries = wb.entries[:0]
	wb.size = 0
	wb.safePoint = 0
	wb.safePointSize = 0
	wb.safePointUndo = 0
}
