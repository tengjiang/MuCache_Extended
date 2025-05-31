package social

import (
	"context"
	"github.com/DKW2/MuCache_Extended/pkg/invoke"
	"github.com/DKW2/MuCache_Extended/pkg/state"
	
	"time"
	//"fmt"
	"strconv"
	"github.com/golang/glog"
)

func ReadUserTimeline(ctx context.Context, userId string) []Post {
	postIds, err := state.GetState[[]string](ctx, userId)
	if err != nil {
		return []Post{}
	}
	req := ReadPostsRequest{PostIds: postIds}
	postsResp := invoke.Invoke[ReadPostsResponse](ctx, "poststorage", "ro_read_posts", req)
	//fmt.Printf("Stored: %+v\nReturned: %+v\n", req, postsResp)
	return postsResp.Posts
}

func ReadSimilarUserTimeline(ctx context.Context, userId string) []Post {
	glog.Infof( "Prefetching User Timeline %v!", userId)
	resp := ReadUserTimeline( ctx, userId )

	// What if synchronous
	// userTimelineIDInt, err := strconv.Atoi(userId[4:])
	// glog.Infof( "Beginning to prefetch user timelines around user timeline %v", userTimelineIDInt )
	// glog.Infof( "Any error: %v", err )
	// if err == nil {
	// 	prefetchIDs := []string{
	// 		"User" + strconv.Itoa(userTimelineIDInt + 1),
	// 		"User" + strconv.Itoa(userTimelineIDInt - 1),
	// 	}

	// 	for _, prefetchID := range prefetchIDs {
	// 		glog.Infof("Prefetching adjacent user timeline: %v", prefetchID)

	// 		// Copy values from the original context
	// 		prefetchCtx := context.Background()
	// 		for _, key := range []interface{}{"read-only", "caller", "RID", "call-args"} {
	// 			if val := ctx.Value(key); val != nil {
	// 				prefetchCtx = context.WithValue(prefetchCtx, key, val)
	// 			}
	// 		}

	// 		ReadUserTimeline( prefetchCtx, prefetchID )
	// 	}
	// }

	// Asynchronous prefetching of adjacent products
	go func() {
		userTimelineIDInt, err := strconv.Atoi(userId[4:])
		glog.Infof( "Beginning to prefetch user timelines around user timeline %v", userTimelineIDInt )
		glog.Infof( "Any error: %v", err )
		if err == nil {
			prefetchIDs := []string{
				"User" + strconv.Itoa(userTimelineIDInt + 1),
				//"User" + strconv.Itoa(userTimelineIDInt - 1),
			}

			for _, prefetchID := range prefetchIDs {
				// Launch a new goroutine for each prefetch request
				glog.Infof( "Starting to create new goroutine for user timeline %v", prefetchID )
				// prefetchCtx := context.Background()
				// prefetchCtx = context.WithValue(prefetchCtx, "read-only", true)
				// prefetchCtx = context.WithValue(prefetchCtx, "caller", ctx.caller)
				// prefetchCtx = context.WithValue(prefetchCtx, "RID", generateCallID())

				glog.Infof( "Starting goroutine to prefetch user timeline %v", prefetchID )
				go func(id string) {
					defer func() {
						if r := recover(); r != nil {
							glog.Warningf("Recovered from panic in prefetch for user timeline %v: %v", prefetchID, r)
						}
					}()
					glog.Infof("Prefetching adjacent user timeline: %v", prefetchID)

					// Copy values from the original context
					mimicCtx := context.Background()

					for _, key := range []interface{}{"read-only", "caller", "RID", "call-args"} {
						if val := ctx.Value(key); val != nil {
							mimicCtx = context.WithValue(mimicCtx, key, val)
						}
					}

					prefetchCtx, cancel := context.WithTimeout(mimicCtx, 5*time.Millisecond)
					defer cancel()

					ReadUserTimeline( prefetchCtx, id )
				}(prefetchID)
			}
		}
	}()

	return resp
}

func WriteUserTimeline(ctx context.Context, userId string, newPostIds []string) {
	postIds, err := state.GetState[[]string](ctx, userId)
	//fmt.Printf("[WriteUserTimeline] old postIds: %+v\n", postIds)
	//fmt.Printf("[WriteUserTimeline] to store: %+v\n", newPostIds)
	if err != nil {
		postIds = []string{}
	}
	if len(postIds) >= 10 {
		postIds = postIds[1:]
	}
	postIds = append(postIds, newPostIds...)
	//fmt.Printf("[WriteUserTimeline] new postIds: %+v\n", postIds)
	state.SetState(ctx, userId, postIds)
}
