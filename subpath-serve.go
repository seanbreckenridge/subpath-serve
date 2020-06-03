package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// default port to serve subpath-serve on
const defaultPort = 8050

// paths to ignore from serveFolder
var ignorePaths = [...]string{".git"}

// configuration information
type config struct {
	port        int
	serveFolder string
}

func parseFlags() *config {
	// flag definitions
	port := flag.Int("port", 8050, "port to serve subpath-serve on")
	serveFolder := flag.String("folder", "./serve", "path to serve subpath-serve on")
	// parse flags
	flag.Parse()
	// make sure path is valid
	fileInfo, err := os.Stat(*serveFolder)
	if err != nil {
		log.Fatalf("Error: Folder to serve files from, '%s' does not exist\n", *serveFolder)
	}
	if !fileInfo.IsDir() {
		log.Fatalf("Error: Path '%s' is not a directory", *serveFolder)
	}
	return &config{
		port:        *port,
		serveFolder: *serveFolder,
	}
}

// generates the response for the "/" request
func index() string {
	var indexBuilder strings.Builder
	err := filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// if the filename matches any of the paths in the global ignorePaths
			// skip the directory
			for _, ignore := range ignorePaths {
				if info.Name() == ignore {
					return filepath.SkipDir
				}
			}
			if path != "." {
				// if this is a file
				if info.Mode().IsRegular() {
					// else append to response string
					indexBuilder.WriteString(path)
					indexBuilder.WriteString("\n")
				}
			}
			return nil
		})
	if err != nil {
		panic(err)
	}
	return indexBuilder.String()
}

// returns nil if file could not be found
// else, returns the contents of the file
//
// errors signify an application error (should be converted to 500)
func find(query string) (*string, error) {
	var foundPath *string
	err := filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// if the filename matches any of the paths in the global ignorePaths
			// skip the directory
			for _, ignore := range ignorePaths {
				if info.Name() == ignore {
					return filepath.SkipDir
				}
			}
			if path != "." {
				// if this is a file
				if info.Mode().IsRegular() {
					// the query matches this path
					if strings.HasSuffix(path, query) &&
						query[strings.LastIndex(query, "/")+1:] == info.Name() {
						// if this matches the suffix of the file
						// return the filename
						foundPath = &path
						// return error from os.Walk func to exit once we find file
						return errors.New("early exit os.Walk")
					}
				}
			}
			return nil
		})
	// if os.walk error and not the early exit
	// return the error, since some os error actually happened
	if err != nil && err.Error() != "early exit os.Walk" {
		return nil, err
	}

	// return the filepath/nil if no file was found
	return foundPath, nil
}

func main() {
	config := parseFlags()
	err := os.Chdir(config.serveFolder)
	if err != nil {
		panic(err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintf(w, "%s", index())
		} else {
			// search for the file
			foundPath, err := find(r.URL.Path[1:])
			// if there was an OS error
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%s\n", err.Error())
			} else {
				// if the file couldnt be found
				if foundPath == nil {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "Could not find a match for %s\n", r.URL.Path[1:])
				} else {
					// if the file was found, return the read file
					data, _ := ioutil.ReadFile(*foundPath)
					w.Header().Set("X-FilePath", *foundPath)
					fmt.Fprintf(w, "%s", data)
				}
			}
		}
	})
	log.Printf("subpath-serve serving %s on port %d\n", config.serveFolder, config.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.port), nil))
}
