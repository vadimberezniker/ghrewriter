package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

var (
	listen = flag.String("listen", "0.0.0.0", "Address to listen on.")
	port   = flag.Int("port", 8080, "Port to listen on")
)

func fetchFile(url string) (string, error) {
	f, err := os.CreateTemp("", "ghfixer-*.gz")
	if err != nil {
		return "", err
	}
	defer f.Close()

	rsp, err := http.Get(url)
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	if _, err := io.Copy(f, rsp.Body); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func fixFile(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "gunzip", path)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "gzip", "-n", strings.TrimSuffix(path, ".gz"))
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		origURL := fmt.Sprintf("https:/%s", r.RequestURI)

		log.Printf("handle %s -> %s", r.URL, origURL)

		tmpFile, err := fetchFile(origURL)
		if err != nil {
			log.Printf("could not fetch file %s: %s", origURL, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmpFile)

		if strings.HasSuffix(r.RequestURI, ".gz") {
			if err := fixFile(r.Context(), tmpFile); err != nil {
				log.Printf("could not fix %s: %s", origURL, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		fr, err := os.Open(tmpFile)
		if err != nil {
			log.Printf("could not open temp file %s: %s", tmpFile, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		io.Copy(w, fr)
	})

	listenAddr := fmt.Sprintf("%s:%d", *listen, *port)

	log.Printf("Listening at %s", listenAddr)

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Printf("serve failed: %s", err)
	}
}
