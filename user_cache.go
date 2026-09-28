package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// userCacheTTL 是这条副本允许存在的时间。到期后下一次查询重新读 PostgreSQL。
const userCacheTTL = 30 * time.Second

// userCache 为空表示这次进程不使用缓存。
var userCache *redis.Client

// cachedPublicUser 是放进 Redis 的副本。只有客户端本来就能看见的字段。
type cachedPublicUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Age      int    `json:"age"`
}

func InitUserCache(addr string) error {
	if addr == "" {
		return nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  300 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
		MaxRetries:   1,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return err
	}
	userCache = client
	return nil
}

func closeUserCache() {
	if userCache == nil {
		return
	}
	_ = userCache.Close()
}

func userCacheKey(id int) string {
	return fmt.Sprintf("user:%d", id)
}

func lookupCachedUser(ctx context.Context, id int) (User, bool) {
	if userCache == nil {
		return User{}, false
	}
	raw, err := userCache.Get(ctx, userCacheKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		log.Printf("cache miss: user %d", id)
		return User{}, false
	}
	if err != nil {
		log.Printf("读取用户缓存失败，回退 PostgreSQL: %v", err)
		return User{}, false
	}
	var cached cachedPublicUser
	if err := json.Unmarshal(raw, &cached); err != nil {
		log.Printf("用户缓存无法解析，回退 PostgreSQL: %v", err)
		return User{}, false
	}
	log.Printf("cache hit: user %d", id)
	return User{ID: cached.ID, Username: cached.Username, Age: cached.Age}, true
}

func storeCachedUser(ctx context.Context, user User) {
	if userCache == nil {
		return
	}
	payload, err := json.Marshal(cachedPublicUser{
		ID:       user.ID,
		Username: user.Username,
		Age:      user.Age,
	})
	if err != nil {
		log.Printf("写入用户缓存失败: %v", err)
		return
	}
	if err := userCache.Set(ctx, userCacheKey(user.ID), payload, userCacheTTL).Err(); err != nil {
		log.Printf("写入用户缓存失败，查询结果仍来自 PostgreSQL: %v", err)
	}
}

func invalidateCachedUser(ctx context.Context, id int) {
	if userCache == nil {
		return
	}
	if err := userCache.Del(ctx, userCacheKey(id)).Err(); err != nil {
		log.Printf("删除用户缓存失败，旧副本最多保留 %s: %v", userCacheTTL, err)
		return
	}
	log.Printf("cache invalidate: user %d", id)
}
