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
	authorizations authorizationSlice
	bucket         string
	address        string
)

func main() {
	flag.Var(&authorizations, "authorize", "usernames:passwords to be used to authenticate (e.g.: carlos:asd123)")
	flag.StringVar(&bucket, "bucket", "", "bucket name (e.g.: s3://foo)")
	flag.StringVar(&address, "addr", "127.0.0.1:8080", "address to listen to (e.g. 127.0.0.1:9090)")
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

		if !authorize(r) {
			log.Println("unauthorized")
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

	log.Println("listening on", address)
	http.ListenAndServe(address, handler)
}

type authorization struct {
	Username, Password string
}

type authorizationSlice []authorization

func (i *authorizationSlice) String() string {
	var strs []string
	for _, a := range *i {
		strs = append(strs, a.Username)
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

func (i *authorizationSlice) Set(value string) error {
	var parts = strings.Split(value, ":")
	if len(parts) != 2 {
		return fmt.Errorf("must be in the username:password format")
	}
	*i = append(*i, authorization{
		parts[0],
		parts[1],
	})
	return nil
}

func authorize(r *http.Request) bool {
	user, pwd, ok := r.BasicAuth()
	if !ok {
		return false
	}
	for _, auth := range authorizations {
		if auth.Username == user && auth.Password == pwd {
			return true
		}
	}
	return false
}
