package cache

import (
	"github.com/bluele/gcache"
	"time"
)

var GC = gcache.New(200).Build()

// 將token保存並設置有效期
func SetWithExpire(Key string, Value interface{}, seconds int) {
	GC.SetWithExpire(Key, Value, time.Second*time.Duration(seconds))
}

// 取得指定token的數據
func CacheGet(key string) (value interface{}, err error) {
	value, err = GC.Get(key)
	return
}

// 將指定的key從cache移除
func CacheRemove(key string) bool {
	ok := GC.Remove(key)
	return ok
}

// 取得全部cache的數據
func GetAllCache() map[interface{}]interface{} {
	return GC.GetALL(true)
}
