package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// default port to serve subpath-serve on
const defaultPort = 8050

const templateName = "dark"

// paths to ignore from serveFolder
var ignorePaths = [...]string{".git"}

// configuration information
type config struct {
	port        int
	serveFolder string
}

type PageInfo struct {
	Title        string
	PageContents string
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

func setupTemplate() *template.Template {
	tmpl, err := template.New(templateName).Parse(`<!DOCTYPE html>
<html class="no-js" lang="en">
<head><meta charset="utf-8"><style>
html, body {
         margin: 0px;
         padding: 0px;
         border: 0px;
         width: 100vw;
         min-height: 100vh;
         background-color: #111;
         color: white;
     }
     main {
         display: flex;
         justify-content: center;
     }
     .container {
         width: 90%;
         margin: 2rem;
         font-family: "Courier", sans-serif;
     }
     pre {
         background-color: #1d2330;
         margin: 1rem;
         padding: 1rem;
         border-radius: min(0.25rem, 15px);
     }
     .title {
         display: flex;
         flex-direction: row;
         justify-content: space-between;
         width: 90%;
         margin-left: auto;
         margin-right: auto;
     }
     code {
         font-size: 120%;
         white-space: pre-wrap; /* css-3 */
         white-space: -moz-pre-wrap; /* Mozilla, since 1999 */
         white-space: -pre-wrap; /* Opera 4-6 */
         white-space: -o-pre-wrap; /* Opera 7 */
         word-wrap: break-word; /* Internet Explorer 5.5+ */
     }
    </style>
    <title>{{ .Title }}</title>
</head>
<body>
    <main>
        <div class="container">
            <div class="title">
                <a href="https://gitlab.com/seanbreckenridge/dotfiles.git">Dotfiles Index</a>
                <a href="#" onclick="RawFile()">Raw</a>
            </div>
            <pre>
                <code>
{{ .PageContents }}
                </code>
            </pre>
        </div>
    </main>
    <script>
        function RawFile() {
            window.location.href = window.location.href.split("?")[0]
        }
    </script>
</body>
</html>
`)
	if err != nil {
		panic(err)
	}
	return tmpl
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

// is dark req specifies whether or not this is a
// plain text response or rendered dark response
func render(w *http.ResponseWriter, info *PageInfo, tmpl *template.Template, isDarkReq bool) {
	if isDarkReq {
		tmpl.Execute(*w, *info)
	} else {
		fmt.Fprintf(*w, "%s", (*info).PageContents)
	}
}

func main() {
	config := parseFlags()
	tmpl := setupTemplate()
	err := os.Chdir(config.serveFolder)
	if err != nil {
		panic(err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		isDark := false
		if _, ok := r.URL.Query()["dark"]; ok {
			isDark = true
		}
		r.URL.Query()
		if r.URL.Path == "/" {
			render(&w, &PageInfo{
				PageContents: index(),
				Title:        "Index",
			}, tmpl, isDark)
		} else {
			// search for the file
			foundPath, err := find(r.URL.Path[1:])
			// if there was an OS error
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				render(&w, &PageInfo{
					PageContents: err.Error(),
					Title:        "Server Error",
				}, tmpl, isDark)
			} else {
				// if the file couldnt be found
				if foundPath == nil {
					w.WriteHeader(http.StatusNotFound)
					render(&w, &PageInfo{
						PageContents: fmt.Sprintf("Could not find a match for %s\n", r.URL.Path[1:]),
						Title:        "404 - Not Found",
					}, tmpl, isDark)
				} else {
					// if the file was found, return the read file
					data, _ := ioutil.ReadFile(*foundPath)
					w.Header().Set("X-Filepath", *foundPath)
					render(&w, &PageInfo{
						PageContents: string(data),
						Title:        *foundPath,
					}, tmpl, isDark)
				}
			}
		}
	})
	log.Printf("subpath-serve serving %s on port %d\n", config.serveFolder, config.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.port), nil))
}
