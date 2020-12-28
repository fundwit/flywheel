package servehttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func StartHTTPServer(engine *gin.Engine) {
	srv := &http.Server{
		Addr:    ":8080",
		Handler: engine,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// will call os.Exit(1)
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 send syscall.SIGINT
	// kill -9 send syscall.SIGKILL, can't be caught
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[QUIT] shutdown signal has been received, the service will exit in 3 seconds.")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// graceful shutdown http.Server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[QUIT] http server shutdown failed: %v\n", err)
	}
	log.Println("[QUIT] http server is shutdown gracefully, new request will be rejected.")

	// waiting for ctx.Done().
	select {
	case <-ctx.Done():
	}
	log.Println("[QUIT] service exiting")
}
