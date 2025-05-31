package movie

import (
	"context"
	"github.com/DKW2/MuCache_Extended/pkg/invoke"
	"time"
	"fmt"
	"strconv"
	"github.com/google/uuid"
	"github.com/golang/glog"
)

func ReadPage(ctx context.Context, movieId string) Page {
	req1 := ReadMovieInfoRequest{MovieId: movieId}
	//fmt.Printf("[Page] Movie id asked: %v\n", movieId)
	movieInfoRes := invoke.Invoke[ReadMovieInfoResponse](ctx, "movieinfo", "ro_read_movie_info", req1)
	movieInfo := movieInfoRes.Info

	// TODO: Make them async
	req2 := ReadCastInfosRequest{CastIds: movieInfo.CastIds}
	castInfosRes := invoke.Invoke[ReadCastInfosResponse](ctx, "castinfo", "ro_read_cast_infos", req2)
	req3 := ReadPlotRequest{PlotId: movieInfo.PlotId}
	plotRes := invoke.Invoke[ReadPlotResponse](ctx, "plot", "ro_read_plot", req3)
	req4 := ReadMovieReviewsRequest{MovieId: movieId}
	reviewsRes := invoke.Invoke[ReadMovieReviewsResponse](ctx, "moviereviews", "ro_read_movie_reviews", req4)
	//fmt.Printf("[Page] Reviews read: %v\n", reviewsRes)
	page := Page{
		MovieInfo: movieInfo,
		CastInfos: castInfosRes.Infos,
		Plot:      plotRes.Plot,
		Reviews:   reviewsRes.Reviews,
	}
	return page
}

func generateCallID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String())
}

func PrefetchReadPage(ctx context.Context, movieId string) Page {
	glog.Infof( "Reading Page %v", movieId)
	res := ReadPage(ctx, movieId)

	// Asynchronous prefetching of adjacent products
	glog.Infof( "Starting goroutine to prefetch pages" )
	go func() {
		movieIDInt, err := strconv.Atoi(movieId)
		glog.Infof( "Beginning to prefetch pages around page %v", movieIDInt )
		glog.Infof( "Any error: %v", err )
		if err == nil {
			prefetchIDs := []string{
				strconv.Itoa(movieIDInt + 1),
				//strconv.Itoa(movieIDInt - 1),
			}

			for _, prefetchID := range prefetchIDs {
				// Launch a new goroutine for each prefetch request
				glog.Infof( "Starting to create new goroutine for page %v", prefetchID )
				// prefetchCtx := context.Background()
				// prefetchCtx = context.WithValue(prefetchCtx, "read-only", true)
				// prefetchCtx = context.WithValue(prefetchCtx, "caller", ctx.caller)
				// prefetchCtx = context.WithValue(prefetchCtx, "RID", generateCallID())

				glog.Infof( "Starting goroutine to prefetch page %v", prefetchID )
				go func(id string) {
					defer func() {
						if r := recover(); r != nil {
							glog.Warningf("Recovered from panic in prefetch for page %v: %v", prefetchID, r)
						}
					}()
					glog.Infof("Prefetching adjacent page: %v", prefetchID)

					// prefetchCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
					// defer cancel()

					ReadPage( ctx, id )
				}(prefetchID)
			}
		}
	}()

	return res
}
