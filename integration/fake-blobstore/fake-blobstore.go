package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	assets := flag.String("assets", "assets", "Asset Directory")
	host := flag.String("host", "127.0.0.1", "Host")
	port := flag.Int("port", 8081, "Port")

	flag.Parse()

	http.HandleFunc("/get_package/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close() //nolint:errcheck

		asset := r.URL.Path[len("/get_package/"):]

		if r.URL.Query().Get("encrypted") != "" {
			if r.Header.Get("encryption-key") == "" {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("no headers")) //nolint:errcheck
				return
			}
		}

		f, err := os.Open(filepath.Join(*assets, asset))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error())) //nolint:errcheck
			return
		}
		defer f.Close() //nolint:errcheck

		_, err = io.Copy(w, f)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error())) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/upload_package/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close() //nolint:errcheck

		asset := r.URL.Path[len("/upload_package/"):]

		f, err := os.Create(filepath.Join(*assets, asset))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error())) //nolint:errcheck
			return
		}
		defer f.Close() //nolint:errcheck

		_, err = io.Copy(f, r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error())) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), nil)
	if err != nil {
		panic(err)
	}
}
