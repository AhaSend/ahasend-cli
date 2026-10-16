// Command fake-release serves release assets the way GitHub serves the
// releases of a repository, for the install tests in test/install:
//
//	/latest                   redirects to /tag/<tag>, for the latest tag
//	/latest/download/<asset>  redirects to /download/<tag>/<asset>
//	/tag/<tag>                a page, when a folder has the tag
//	/download/<tag>/<asset>   <dir>/<tag>/<asset> from the first <dir> that has it
//
// Usage:
//
//	fake-release -url <file> -latest <file> [-slow <tag>] [-addr <address>] <dir>...
//
// It reads the latest tag from the -latest file on every request, so a test
// can change it; without that file, /latest is a 404. A download of the -slow
// tag sends a few bytes, waits three seconds and breaks off, so a test can
// stop an install script during a download. The server listens on a free
// port of 127.0.0.1, or on the -addr address when the files under test
// already hold the server's URL, writes its base URL into the -url file once
// it is ready, and logs every request to stderr.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	urlFile := flag.String("url", "", "the file to write the base URL into")
	latestFile := flag.String("latest", "", "the file that holds the latest tag")
	slowTag := flag.String("slow", "", "the tag whose downloads break off")
	addr := flag.String("addr", "127.0.0.1:0", "the address to listen on")
	flag.Parse()
	if *urlFile == "" || *latestFile == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: fake-release -url <file> -latest <file> [-slow <tag>] [-addr <address>] <dir>...")
		os.Exit(2)
	}
	dirs := flag.Args()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	base := "http://" + listener.Addr().String()

	// find returns the path of <dir>/<tag>/<name> in the first folder that has
	// it, or "" when none has; with an empty name, it finds the tag's folder
	find := func(tag, name string) string {
		for _, dir := range dirs {
			path := filepath.Join(dir, tag, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		return ""
	}
	latest := func() string {
		data, err := os.ReadFile(*latestFile)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /latest", func(w http.ResponseWriter, r *http.Request) {
		tag := latest()
		if tag == "" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, base+"/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("GET /latest/download/{asset}", func(w http.ResponseWriter, r *http.Request) {
		tag := latest()
		if tag == "" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, base+"/download/"+tag+"/"+r.PathValue("asset"), http.StatusFound)
	})
	mux.HandleFunc("GET /tag/{tag}", func(w http.ResponseWriter, r *http.Request) {
		tag := r.PathValue("tag")
		if find(tag, "") == "" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "Release %s\n", tag)
	})
	mux.HandleFunc("GET /download/{tag}/{asset}", func(w http.ResponseWriter, r *http.Request) {
		tag := r.PathValue("tag")
		if tag == *slowTag {
			// Fewer bytes than Content-Length breaks off the connection
			w.Header().Set("Content-Length", "1048576")
			_, _ = w.Write(make([]byte, 1024))
			w.(http.Flusher).Flush()
			time.Sleep(3 * time.Second)
			return
		}
		path := find(tag, r.PathValue("asset"))
		if path == "" {
			http.NotFound(w, r)
			return
		}
		// GitHub serves every asset with this type
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	})

	// Renamed into place, so a test never reads half of it
	if err := os.WriteFile(*urlFile+".tmp", []byte(base+"\n"), 0o644); err != nil {
		log.Fatal(err)
	}
	if err := os.Rename(*urlFile+".tmp", *urlFile); err != nil {
		log.Fatal(err)
	}
	logRequests := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})
	log.Fatal(http.Serve(listener, logRequests))
}
