package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/httperr"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"gocloud.dev/blob"

	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
)

var (
	auths  stringSlice
	bucket string
	listen string
)

func main() {
	flag.Var(&auths, "authorize", "user/passwords that can authenticate in the user:pwd format (e.g.: carlos:asd123)")
	flag.StringVar(&bucket, "bucket", "", "bucket name (e.g.: s3://foo)")
	flag.StringVar(&listen, "listen", "127.0.0.1:8080", "address to listen to (e.g. 127.0.0.1:9090)")
	flag.Parse()

	ctx := context.Background()
	bucket, err := blob.OpenBucket(ctx, bucket)
	if err != nil {
		log.Fatalln(err)
	}
	defer bucket.Close()

	var r = chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	r.Get("/{asset}", httperr.NewF(func(w http.ResponseWriter, r *http.Request) error {
		var path = chi.URLParam(r, "asset")

		user, pwd, ok := r.BasicAuth()
		if !(ok && isAuthorized(user+":"+pwd)) {
			log.Println("unauthorized:", user)
			return httperr.Wrap(fmt.Errorf("missing/invalid authorization"), http.StatusUnauthorized)
		}

		reader, err := bucket.NewReader(ctx, path, nil)
		if err != nil {
			return httperr.Wrap(err, http.StatusNotFound)
		}
		defer reader.Close()
		if _, err := io.Copy(w, reader); err != nil {
			return httperr.Wrap(err, http.StatusInternalServerError)
		}
		return nil
	}).ServeHTTP)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    listen,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Println("listening on", listen)

	<-done
	log.Println("stopping")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("couldn't stop server: %+v", err)
	}
}

type stringSlice []string

func (i *stringSlice) String() string {
	return "[" + strings.Join(*i, ", ") + "]"
}

func (i *stringSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func isAuthorized(input string) bool {
	for _, auth := range auths {
		if input == auth {
			return true
		}
	}
	return false
}
