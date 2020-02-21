package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/caarlos0/httperr"
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

	var handler = http.NewServeMux()

	handler.Handle("/", httperr.NewF(func(w http.ResponseWriter, r *http.Request) error {
		var path = strings.Replace(r.URL.EscapedPath(), "/", "", 1)
		log.Println(path)

		user, pwd, ok := r.BasicAuth()
		if !(ok && isAuthorized(user+":"+pwd)) {
			log.Println("unauthorized:", user)
			return httperr.Wrap(fmt.Errorf("missing/invalid authorization"), http.StatusUnauthorized)
		}

		reader, err := bucket.NewReader(ctx, path, nil)
		if err != nil {
			return httperr.Wrap(err, http.StatusBadRequest)
		}
		defer reader.Close()
		if _, err := io.Copy(w, reader); err != nil {
			return httperr.Wrap(err, http.StatusInternalServerError)
		}
		return nil
	}))

	log.Println("listening on", listen)
	http.ListenAndServe(listen, handler)
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
